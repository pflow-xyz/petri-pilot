/**
 * pflow-engine.js — Unified Petri net runtime
 *
 * Combines ODE solving (from petri-solver.js / pflow-xyz) with discrete
 * event-sourced execution (ported from go-pflow/eventsource).
 *
 * Two execution modes from one model:
 *   engine.ode(rates, tspan)   — continuous ODE simulation
 *   engine.fire(id, transition, data) — discrete event sourcing
 */

import { expandColors, expandState } from './petri-colors.js';

// ============================================================================
// Data Structures (shared with petri-solver.js)
// ============================================================================

export class Place {
  constructor(label, initial = [], capacity = [], x = 0, y = 0, labelText = null) {
    this.label = label;
    this.initial = Array.isArray(initial) ? initial : [initial];
    this.capacity = Array.isArray(capacity) ? capacity : [capacity];
    this.x = x;
    this.y = y;
    this.labelText = labelText;
  }

  getTokenCount() {
    return this.initial.length === 0 ? 0 : this.initial.reduce((a, b) => a + b, 0);
  }
}

export class Transition {
  constructor(label, role = 'default', x = 0, y = 0, labelText = null) {
    this.label = label;
    this.role = role;
    this.x = x;
    this.y = y;
    this.labelText = labelText;
  }
}

export class Arc {
  constructor(source, target, weight = [1], inhibit = false) {
    this.source = source;
    this.target = target;
    this.weight = Array.isArray(weight) ? weight : [weight];
    this.inhibit = inhibit;
  }

  getWeightSum() {
    return this.weight.length === 0 ? 1 : this.weight.reduce((a, b) => a + b, 0);
  }
}

// ============================================================================
// PetriNet — the core model (compatible with petri-solver.js API)
// ============================================================================

export class PetriNet {
  constructor() {
    this.places = new Map();
    this.transitions = new Map();
    this.arcs = [];
    this.token = [];
  }

  addPlace(label, initial, capacity, x, y, labelText) {
    const place = new Place(label, initial, capacity, x, y, labelText);
    this.places.set(label, place);
    return place;
  }

  addTransition(label, role, x, y, labelText) {
    const transition = new Transition(label, role, x, y, labelText);
    this.transitions.set(label, transition);
    return transition;
  }

  addArc(source, target, weight, inhibit = false) {
    const arc = new Arc(source, target, weight, inhibit);
    this.arcs.push(arc);
    return arc;
  }

  /** Get input arcs for a transition (place → transition) */
  inputArcs(transitionId) {
    return this.arcs.filter(
      a => a.target === transitionId && this.places.has(a.source)
    );
  }

  /** Get output arcs for a transition (transition → place) */
  outputArcs(transitionId) {
    return this.arcs.filter(
      a => a.source === transitionId && this.places.has(a.target)
    );
  }

  /** Get inhibitor arcs for a transition */
  inhibitorArcs(transitionId) {
    return this.arcs.filter(
      a => a.target === transitionId && a.inhibit
    );
  }
}

// ============================================================================
// Model Loaders — parse model.json formats
// ============================================================================

/**
 * Parse pflow-xyz JSON-LD / petri-solver.js format (object-keyed places/transitions)
 */
export function fromJSON(data) {
  if (typeof data === 'string') data = JSON.parse(data);
  const net = new PetriNet();
  if (data.token) net.token = data.token;

  if (data.places) {
    for (const [label, p] of Object.entries(data.places)) {
      net.addPlace(label, p.initial || [], p.capacity || [], p.x || 0, p.y || 0, p.label || null);
    }
  }
  if (data.transitions) {
    for (const [label, t] of Object.entries(data.transitions)) {
      net.addTransition(label, t.role || 'default', t.x || 0, t.y || 0, t.label || null);
    }
  }
  if (data.arcs) {
    for (const a of data.arcs) {
      net.addArc(a.source, a.target, a.weight || [1], a.inhibitTransition || false);
    }
  }
  return net;
}

/**
 * Parse petri-pilot model.json format (array-based places/transitions with from/to arcs)
 */
export function fromModelJSON(data) {
  if (typeof data === 'string') data = JSON.parse(data);

  // Handle v2 format with net wrapper
  const netData = data.net || data;
  const net = new PetriNet();

  // Places are arrays of { id, description, initial }
  if (netData.places) {
    for (const p of netData.places) {
      const initial = (p.initial != null && p.initial > 0) ? [p.initial] : [];
      net.addPlace(p.id, initial, [], p.x || 0, p.y || 0, p.description || null);
    }
  }

  // Transitions are arrays of { id, description, rate }
  if (netData.transitions) {
    for (const t of netData.transitions) {
      net.addTransition(t.id, t.role || 'default', t.x || 0, t.y || 0, t.description || null);
    }
  }

  // Arcs use from/to keys instead of source/target
  if (netData.arcs) {
    for (const a of netData.arcs) {
      const weight = a.weight ? (Array.isArray(a.weight) ? a.weight : [a.weight]) : [1];
      net.addArc(a.from || a.source, a.to || a.target, weight, a.inhibit || false);
    }
  }

  return net;
}

/**
 * Auto-detect format and parse
 */
export function loadModel(data) {
  if (typeof data === 'string') data = JSON.parse(data);

  // CompositeNet — cannot be loaded as a single net
  if (data['@type'] === 'CompositeNet' || data.schemas) {
    throw new Error('CompositeNet detected — use CompositeModel from pflow-composer.js');
  }

  // petri-pilot v2 format: has .net.places as array
  if (data.net && Array.isArray(data.net.places)) {
    return fromModelJSON(data);
  }
  // petri-pilot v1 format: has .places as array
  if (Array.isArray(data.places)) {
    return fromModelJSON(data);
  }
  // pflow-xyz / petri-solver.js format: has .places as object
  return fromJSON(data);
}

// ============================================================================
// ODE Solver (from petri-solver.js)
// ============================================================================

export function setState(net, customState = null) {
  const state = {};
  for (const [label, place] of net.places) {
    state[label] = (customState && customState[label] !== undefined)
      ? customState[label]
      : place.getTokenCount();
  }
  return state;
}

export function setRates(net, customRates = null) {
  const rates = {};
  for (const [label] of net.transitions) {
    rates[label] = (customRates && customRates[label] !== undefined)
      ? customRates[label]
      : 1.0;
  }
  return rates;
}

function buildODEFunction(net, rates) {
  return function (t, u) {
    const du = {};
    for (const label of net.places.keys()) du[label] = 0.0;

    for (const [transLabel] of net.transitions) {
      const rate = rates[transLabel];
      let flux = rate;

      for (const arc of net.arcs) {
        if (arc.target === transLabel && net.places.has(arc.source)) {
          const placeState = u[arc.source];
          if (placeState <= 0) { flux = 0; break; }
          flux *= placeState;
        }
      }

      if (flux > 0) {
        for (const arc of net.arcs) {
          const weight = arc.getWeightSum();
          if (arc.target === transLabel && net.places.has(arc.source)) {
            du[arc.source] -= flux * weight;
          } else if (arc.source === transLabel && net.places.has(arc.target)) {
            du[arc.target] += flux * weight;
          }
        }
      }
    }
    return du;
  };
}

export class ODEProblem {
  /**
   * Multi-color nets are unfolded first (expandColors), so mass action runs per
   * color: a transition's flux depends only on the colors its input arcs name,
   * and consumes only those. Summing the vectors would let a pool of blue
   * tokens drive a red-only reaction.
   *
   * initialState is mapped through expandState. Results still report base
   * names by default — see ODESolution.
   */
  constructor(net, initialState, tspan, rates) {
    const { net: expanded, colorMap } = expandColors(net);
    if (colorMap !== null) {
      initialState = expandState(net, initialState);
      net = expanded;
    }
    this.colorMap = colorMap;
    this.net = net;
    this.u0 = initialState;
    this.tspan = tspan;
    this.rates = rates;
    this.f = buildODEFunction(net, rates);
  }
}

export class ODESolution {
  /**
   * On a color-unfolded problem `u` and `stateLabels` use the expanded
   * per-color place names ("pool.red"); getFinalState and getState fold them
   * back to per-place totals under the original names, and getVariable accepts
   * either. Matches go-pflow's solver.Solution.
   */
  constructor(t, u, stateLabels, colorMap = null) {
    this.t = t;
    this.u = u;
    this.stateLabels = stateLabels;
    this.colorMap = colorMap;
  }

  /** A base name sums the colors; an expanded name picks one out. */
  getVariable(index) {
    const label = typeof index === 'number' ? this.stateLabels[index] : index;
    const labels = this.colorMap ? this.colorMap.lookup(label) : [label];
    return this.u.map(state => {
      let sum = 0;
      for (const l of labels) sum += state[l] ?? 0;
      return sum;
    });
  }

  /** Per-color series for a base place, index-aligned with colorMap.colors. */
  getVariableByColor(place) {
    const labels = this.colorMap ? this.colorMap.lookup(place) : [place];
    return labels.map(l => this.u.map(state => state[l] ?? 0));
  }

  /** Keyed by the original place names; colors are summed. */
  getFinalState() {
    const last = this.u[this.u.length - 1];
    if (last === undefined) return last;
    return this.colorMap ? this.colorMap.sumByBase(last) : last;
  }

  /** Keyed by expanded per-color place names. */
  getFinalStateByColor() {
    return this.u[this.u.length - 1];
  }

  /** Keyed by the original place names; colors are summed. */
  getState(index) {
    const st = this.u[index];
    if (st === undefined) return st;
    return this.colorMap ? this.colorMap.sumByBase(st) : st;
  }

  /** Keyed by expanded per-color place names. */
  getStateByColor(index) {
    return this.u[index];
  }
}

export function Tsit5() {
  return {
    name: 'Tsit5',
    order: 5,
    c: [0, 0.161, 0.327, 0.9, 0.9800255409045097, 1, 1],
    a: [
      [],
      [0.161],
      [-0.008480655492356924, 0.335480655492357],
      [2.8971530571054935, -6.359448489975075, 4.362295432869581],
      [5.325864828439257, -11.748883564062828, 7.4955393428898365, -0.09249506636175525],
      [5.86145544294642, -12.92096931784711, 8.159367898576159, -0.071584973281401, -0.028269050394068383],
      [0.09646076681806523, 0.01, 0.4798896504144996, 1.379008574103742, -3.290069515436081, 2.324710524099774, 0],
    ],
    b: [0.09646076681806523, 0.01, 0.4798896504144996, 1.379008574103742, -3.290069515436081, 2.324710524099774, 0],
    bhat: [0.001780011052226, 0.000816434459657, -0.007880878010262, 0.144711007173263, -0.582357165452555, 0.458082105929187, 1.0 / 66.0],
  };
}

export function solve(prob, solver = Tsit5(), options = {}) {
  const {
    dt = 0.01, dtmin = 1e-6, dtmax = 0.1,
    abstol = 1e-6, reltol = 1e-3,
    maxiters = 100000, adaptive = true,
  } = options;

  const [t0, tf] = prob.tspan;
  const stateLabels = Object.keys(prob.u0);
  const t = [t0];
  const u = [{ ...prob.u0 }];

  let tcur = t0;
  let ucur = { ...prob.u0 };
  let dtcur = dt;
  let nsteps = 0;

  while (tcur < tf && nsteps < maxiters) {
    if (tcur + dtcur > tf) dtcur = tf - tcur;

    const k = [];
    k[0] = prob.f(tcur, ucur);

    for (let stage = 1; stage < solver.c.length; stage++) {
      const tstage = tcur + solver.c[stage] * dtcur;
      const ustage = {};
      for (const key of stateLabels) {
        ustage[key] = ucur[key];
        for (let j = 0; j < stage; j++) {
          ustage[key] += dtcur * solver.a[stage][j] * k[j][key];
        }
      }
      k[stage] = prob.f(tstage, ustage);
    }

    const unext = {};
    for (const key of stateLabels) {
      unext[key] = ucur[key];
      for (let j = 0; j < solver.b.length; j++) {
        unext[key] += dtcur * solver.b[j] * k[j][key];
      }
    }

    let err = 0;
    if (adaptive) {
      for (const key of stateLabels) {
        let errest = 0;
        for (let j = 0; j < solver.bhat.length; j++) {
          errest += dtcur * solver.bhat[j] * k[j][key];
        }
        const scale = abstol + reltol * Math.max(Math.abs(ucur[key]), Math.abs(unext[key]));
        err = Math.max(err, Math.abs(errest) / scale);
      }
    }

    if (!adaptive || err <= 1.0 || dtcur <= dtmin) {
      tcur += dtcur;
      ucur = unext;
      t.push(tcur);
      u.push({ ...ucur });
      nsteps++;
      if (adaptive && err > 0) {
        const factor = 0.9 * Math.pow(1.0 / err, 1.0 / (solver.order + 1));
        dtcur = Math.min(dtmax, Math.max(dtmin, dtcur * Math.min(factor, 5.0)));
      }
    } else {
      const factor = 0.9 * Math.pow(1.0 / err, 1.0 / (solver.order + 1));
      dtcur = Math.max(dtmin, dtcur * Math.max(factor, 0.1));
    }
  }

  return new ODESolution(t, u, stateLabels, prob.colorMap ?? null);
}

// ============================================================================
// SVG Plotter (from petri-solver.js)
// ============================================================================

export class SVGPlotter {
  constructor(width = 600, height = 400) {
    this.width = width;
    this.height = height;
    this.margin = { top: 40, right: 30, bottom: 50, left: 60 };
    this.plotWidth = width - this.margin.left - this.margin.right;
    this.plotHeight = height - this.margin.top - this.margin.bottom;
    this.title = '';
    this.xlabel = 'Time';
    this.ylabel = 'Value';
    this.series = [];
  }

  setTitle(t) { this.title = t; return this; }
  setXLabel(l) { this.xlabel = l; return this; }
  setYLabel(l) { this.ylabel = l; return this; }

  addSeries(x, y, label = '', color = null) {
    if (!color) {
      const colors = ['#e41a1c','#377eb8','#4daf4a','#984ea3','#ff7f00','#ffff33','#a65628','#f781bf'];
      color = colors[this.series.length % colors.length];
    }
    this.series.push({ x, y, label, color });
    return this;
  }

  render() {
    let xmin = Infinity, xmax = -Infinity, ymin = Infinity, ymax = -Infinity;
    for (const s of this.series) {
      for (let i = 0; i < s.x.length; i++) {
        xmin = Math.min(xmin, s.x[i]); xmax = Math.max(xmax, s.x[i]);
        ymin = Math.min(ymin, s.y[i]); ymax = Math.max(ymax, s.y[i]);
      }
    }
    const xrange = xmax - xmin || 1;
    const yrange = ymax - ymin || 1;
    xmin -= xrange * 0.05; xmax += xrange * 0.05;
    ymin -= yrange * 0.1; ymax += yrange * 0.1;

    const sx = x => this.margin.left + ((x - xmin) / (xmax - xmin)) * this.plotWidth;
    const sy = y => this.margin.top + this.plotHeight - ((y - ymin) / (ymax - ymin)) * this.plotHeight;

    const plotId = 'plot_' + Math.random().toString(36).substr(2, 9);
    let svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${this.width}" height="${this.height}" style="background:white;" id="${plotId}">`;

    if (this.title) {
      svg += `<text x="${this.width/2}" y="25" text-anchor="middle" font-family="Arial,sans-serif" font-size="16" font-weight="bold">${this.title}</text>`;
    }

    svg += `<line x1="${this.margin.left}" y1="${this.margin.top}" x2="${this.margin.left}" y2="${this.margin.top+this.plotHeight}" stroke="#333" stroke-width="2"/>`;
    svg += `<line x1="${this.margin.left}" y1="${this.margin.top+this.plotHeight}" x2="${this.margin.left+this.plotWidth}" y2="${this.margin.top+this.plotHeight}" stroke="#333" stroke-width="2"/>`;
    svg += `<text x="${this.margin.left+this.plotWidth/2}" y="${this.height-10}" text-anchor="middle" font-family="Arial,sans-serif" font-size="12">${this.xlabel}</text>`;
    svg += `<text x="15" y="${this.margin.top+this.plotHeight/2}" text-anchor="middle" font-family="Arial,sans-serif" font-size="12" transform="rotate(-90,15,${this.margin.top+this.plotHeight/2})">${this.ylabel}</text>`;

    for (let i = 0; i <= 5; i++) {
      const x = xmin + (xmax-xmin)*i/5; const px = sx(x);
      svg += `<line x1="${px}" y1="${this.margin.top+this.plotHeight}" x2="${px}" y2="${this.margin.top+this.plotHeight+5}" stroke="#333"/>`;
      svg += `<text x="${px}" y="${this.margin.top+this.plotHeight+20}" text-anchor="middle" font-family="Arial,sans-serif" font-size="10">${x.toFixed(1)}</text>`;
      svg += `<line x1="${px}" y1="${this.margin.top}" x2="${px}" y2="${this.margin.top+this.plotHeight}" stroke="#ddd" stroke-width="0.5"/>`;
    }
    for (let i = 0; i <= 5; i++) {
      const y = ymin + (ymax-ymin)*i/5; const py = sy(y);
      svg += `<line x1="${this.margin.left-5}" y1="${py}" x2="${this.margin.left}" y2="${py}" stroke="#333"/>`;
      svg += `<text x="${this.margin.left-10}" y="${py+4}" text-anchor="end" font-family="Arial,sans-serif" font-size="10">${y.toFixed(1)}</text>`;
      svg += `<line x1="${this.margin.left}" y1="${py}" x2="${this.margin.left+this.plotWidth}" y2="${py}" stroke="#ddd" stroke-width="0.5"/>`;
    }

    for (const s of this.series) {
      let path = 'M';
      for (let i = 0; i < s.x.length; i++) {
        path += (i === 0 ? '' : ' L') + `${sx(s.x[i])},${sy(s.y[i])}`;
      }
      svg += `<path d="${path}" stroke="${s.color}" stroke-width="2" fill="none"/>`;
    }

    if (this.series.some(s => s.label)) {
      let ly = this.margin.top + 10;
      for (const s of this.series) {
        if (s.label) {
          svg += `<line x1="${this.width-this.margin.right-50}" y1="${ly}" x2="${this.width-this.margin.right-30}" y2="${ly}" stroke="${s.color}" stroke-width="2"/>`;
          svg += `<text x="${this.width-this.margin.right-25}" y="${ly+4}" font-family="Arial,sans-serif" font-size="10">${s.label}</text>`;
          ly += 20;
        }
      }
    }

    // Crosshair
    svg += `<g id="${plotId}_crosshair" style="display:none;">`;
    svg += `<line id="${plotId}_line" x1="0" y1="${this.margin.top}" x2="0" y2="${this.margin.top+this.plotHeight}" stroke="#666" stroke-width="1" stroke-dasharray="4,4"/>`;
    svg += `<rect id="${plotId}_tooltip_bg" x="0" y="0" rx="4" ry="4" fill="white" stroke="#666" opacity="0.95"/>`;
    svg += `<text id="${plotId}_tooltip_text" x="0" y="0" font-family="Arial,sans-serif" font-size="11" fill="#333"></text>`;
    svg += `</g>`;
    svg += `<rect id="${plotId}_overlay" x="${this.margin.left}" y="${this.margin.top}" width="${this.plotWidth}" height="${this.plotHeight}" fill="transparent" style="cursor:crosshair;"/>`;
    svg += '</svg>';

    this.lastPlotData = { plotId, margin: this.margin, plotWidth: this.plotWidth, plotHeight: this.plotHeight, xmin, xmax, ymin, ymax, series: this.series };
    return svg;
  }

  static setupInteractivity(plotData) {
    const { plotId, margin, plotWidth, xmin, xmax, ymin, ymax, series } = plotData;
    const svg = document.getElementById(plotId);
    if (!svg) return;
    const crosshair = document.getElementById(plotId + '_crosshair');
    const line = document.getElementById(plotId + '_line');
    const tooltipBg = document.getElementById(plotId + '_tooltip_bg');
    const tooltipText = document.getElementById(plotId + '_tooltip_text');
    const overlay = document.getElementById(plotId + '_overlay');
    if (!crosshair || !overlay) return;

    function getYAtX(s, xval) {
      if (xval <= s.x[0]) return s.y[0];
      if (xval >= s.x[s.x.length-1]) return s.y[s.y.length-1];
      for (let i = 0; i < s.x.length-1; i++) {
        if (xval >= s.x[i] && xval <= s.x[i+1]) {
          const t = (xval - s.x[i]) / (s.x[i+1] - s.x[i]);
          return s.y[i] + t * (s.y[i+1] - s.y[i]);
        }
      }
      return s.y[s.y.length-1];
    }

    overlay.addEventListener('mousemove', function (e) {
      const rect = svg.getBoundingClientRect();
      const mouseX = e.clientX - rect.left;
      crosshair.style.display = 'block';
      line.setAttribute('x1', mouseX);
      line.setAttribute('x2', mouseX);
      const dataX = xmin + (mouseX - margin.left) / plotWidth * (xmax - xmin);
      const lines = ['T = ' + dataX.toFixed(3)];
      for (const s of series) lines.push(s.label + ': ' + getYAtX(s, dataX).toFixed(3));

      const pad = 8, lh = 14, tw = 120, th = lines.length * lh + pad * 2;
      let tx = mouseX + 10;
      if (tx + tw > margin.left + plotWidth) tx = mouseX - tw - 10;
      const ty = margin.top + 10;
      tooltipBg.setAttribute('x', tx); tooltipBg.setAttribute('y', ty);
      tooltipBg.setAttribute('width', tw); tooltipBg.setAttribute('height', th);
      tooltipText.setAttribute('x', tx + pad); tooltipText.setAttribute('y', ty + pad + 12);
      tooltipText.innerHTML = '';
      for (let i = 0; i < lines.length; i++) {
        const tspan = document.createElementNS('http://www.w3.org/2000/svg', 'tspan');
        tspan.textContent = lines[i];
        tspan.setAttribute('x', tx + pad);
        tspan.setAttribute('dy', i === 0 ? '0' : '1.2em');
        if (i === 0) tspan.setAttribute('font-weight', 'bold');
        tooltipText.appendChild(tspan);
      }
    });
    overlay.addEventListener('mouseleave', () => { crosshair.style.display = 'none'; });
  }

  static plotSolution(sol, variables = null, options = {}) {
    const plotter = new SVGPlotter(options.width, options.height);
    if (options.title) plotter.setTitle(options.title);
    if (options.xlabel) plotter.setXLabel(options.xlabel);
    if (options.ylabel) plotter.setYLabel(options.ylabel);
    for (const v of (variables || sol.stateLabels)) plotter.addSeries(sol.t, sol.getVariable(v), v);
    const svg = plotter.render();
    return { svg, plotData: plotter.lastPlotData, setupInteractivity: () => SVGPlotter.setupInteractivity(plotter.lastPlotData) };
  }
}

// ============================================================================
// Discrete Execution — Event-Sourced State Machine
// (Ported from go-pflow/eventsource/aggregate.go)
// ============================================================================

function uuid() {
  return crypto.randomUUID ? crypto.randomUUID() : (
    'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
      const r = Math.random() * 16 | 0;
      return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
    })
  );
}

/**
 * MemoryEventStore — in-memory event store with localStorage persistence.
 * Mirrors go-pflow/eventsource/memory.go
 */
export class MemoryEventStore {
  constructor(storageKey = null) {
    this.streams = new Map();       // aggregateId → Event[]
    this.subscribers = new Set();   // Set<Function>
    this.storageKey = storageKey;

    if (storageKey) {
      try {
        const saved = localStorage.getItem(storageKey);
        if (saved) {
          const parsed = JSON.parse(saved);
          for (const [id, events] of Object.entries(parsed)) {
            this.streams.set(id, events);
          }
        }
      } catch (_) { /* ignore */ }
    }
  }

  _persist() {
    if (!this.storageKey) return;
    try {
      const obj = {};
      for (const [id, events] of this.streams) obj[id] = events;
      localStorage.setItem(this.storageKey, JSON.stringify(obj));
    } catch (_) { /* ignore */ }
  }

  /** Append events to a stream with optimistic concurrency */
  append(streamId, events, expectedVersion) {
    const stream = this.streams.get(streamId) || [];
    if (expectedVersion !== undefined && stream.length !== expectedVersion) {
      throw new Error(`concurrency conflict: expected version ${expectedVersion}, got ${stream.length}`);
    }

    const now = new Date().toISOString();
    for (let i = 0; i < events.length; i++) {
      const event = {
        id: uuid(),
        stream: streamId,
        version: stream.length + 1,
        type: events[i].type,
        timestamp: now,
        data: events[i].data || {},
      };
      stream.push(event);
    }

    this.streams.set(streamId, stream);
    this._persist();

    for (const fn of this.subscribers) {
      try { fn(streamId, stream.slice(-events.length)); } catch (_) { /* ignore */ }
    }
  }

  /** Read events for a stream, optionally from a version */
  read(streamId, fromVersion = 0) {
    const stream = this.streams.get(streamId);
    if (!stream) return [];
    return stream.slice(fromVersion).map(e => ({ ...e }));
  }

  /** Subscribe to new events */
  subscribe(fn) {
    this.subscribers.add(fn);
    return () => this.subscribers.delete(fn);
  }

  /** List all stream IDs */
  listInstances() {
    return [...this.streams.keys()];
  }

  /** Get basic stats */
  getStats() {
    let totalEvents = 0;
    for (const events of this.streams.values()) totalEvents += events.length;
    return {
      total_streams: this.streams.size,
      total_events: totalEvents,
      average_events_per_stream: this.streams.size > 0 ? totalEvents / this.streams.size : 0,
    };
  }

  /** Clear all data */
  clear() {
    this.streams.clear();
    this._persist();
  }
}

// ============================================================================
// PflowEngine — unified engine combining ODE + discrete execution
// ============================================================================

export class PflowEngine {
  /**
   * @param {object} modelJson — raw model.json (petri-pilot or pflow-xyz format)
   * @param {object} options — { storageKey, store }
   */
  constructor(modelJson, options = {}) {
    this.modelJson = typeof modelJson === 'string' ? JSON.parse(modelJson) : modelJson;

    // Multi-color models are unfolded (expandColors), so the discrete engine
    // fires per color: an arc naming red is satisfied by red tokens only, never
    // by a summed pool. State.places is then keyed by the expanded names
    // ("pool.red"), which is what go-pflow's event-sourced marking does too;
    // colorMap recovers the mapping. rawNet keeps the model as authored, which
    // the ODE path needs so its results can report base names.
    this.rawNet = loadModel(this.modelJson);
    const { net: expandedNet, colorMap } = expandColors(this.rawNet);
    this.net = expandedNet;
    this.colorMap = colorMap;

    // JSON-LD metadata (v2.1)
    this.context = this.modelJson['@context'] || null;
    this.type = this.modelJson['@type'] || null;
    this.id = this.modelJson['@id'] || null;

    // Extensions from model.json
    const ext = this.modelJson.extensions || {};
    this.entities = ext['petri-pilot/entities'] || [];
    this.roles = ext['petri-pilot/roles'] || [];
    this.views = ext['petri-pilot/views'] || null;
    this.pages = ext['petri-pilot/pages'] || null;

    // Schema metadata
    const netData = this.modelJson.net || this.modelJson;
    this.name = netData.name || 'unknown';
    this.description = netData.description || '';

    // Event store
    this.store = options.store || new MemoryEventStore(options.storageKey || null);

    // Composition hooks — called after a successful fire()
    this._onFireCallbacks = [];

    // Build transition map for fast lookup: { transitionId: { inputs, outputs } }
    this._transitionMap = new Map();
    for (const [tId] of this.net.transitions) {
      this._transitionMap.set(tId, {
        inputs: this.net.inputArcs(tId),
        outputs: this.net.outputArcs(tId),
        inhibitors: this.net.inhibitorArcs(tId),
      });
    }
  }

  // ── Initial state from model ──

  _initialPlaces() {
    const places = {};
    for (const [label, place] of this.net.places) {
      places[label] = place.getTokenCount();
    }
    return places;
  }

  // ── State reconstruction (replay events) ──

  _replayState(streamId) {
    const events = this.store.read(streamId);
    const state = {
      places: this._initialPlaces(),
      data: {},
      version: 0,
      created_at: null,
    };

    for (const event of events) {
      this._applyEvent(state, event);
    }
    return state;
  }

  _applyEvent(state, event) {
    state.version = event.version;
    if (!state.created_at) state.created_at = event.timestamp;

    // Merge event data into state data
    if (event.data) {
      for (const [k, v] of Object.entries(event.data)) {
        if (k !== 'aggregate_id') state.data[k] = v;
      }
    }

    // Find transition and apply token changes
    const transId = this._eventTypeToTransition(event.type);
    if (!transId) return;

    const info = this._transitionMap.get(transId);
    if (!info) return;

    for (const arc of info.inputs) {
      const w = arc.getWeightSum();
      state.places[arc.source] = (state.places[arc.source] || 0) - w;
    }
    for (const arc of info.outputs) {
      const w = arc.getWeightSum();
      state.places[arc.target] = (state.places[arc.target] || 0) + w;
    }
  }

  /** Convert event type back to transition ID. Event types are PascalCase of transition IDs. */
  _eventTypeToTransition(eventType) {
    // Try direct match first (transition ID === event type)
    if (this._transitionMap.has(eventType)) return eventType;

    // Try converting PascalCase to snake_case
    const snake = eventType.replace(/([a-z])([A-Z])/g, '$1_$2').toLowerCase();
    // Remove trailing 'ed' or 'd' suffix that Go codegen adds
    for (const suffix of ['ed', 'd', '']) {
      const candidate = suffix ? snake.replace(new RegExp(suffix + '$'), '') : snake;
      if (this._transitionMap.has(candidate)) return candidate;
    }

    // Brute force: try matching transition IDs
    for (const [tId] of this._transitionMap) {
      const tPascal = tId.split('_').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join('');
      if (tPascal === eventType || tPascal + 'ed' === eventType || tPascal + 'd' === eventType) {
        return tId;
      }
    }
    return null;
  }

  /** Convert transition ID to event type (PascalCase + 'ed') matching Go codegen */
  _transitionToEventType(transId) {
    const pascal = transId.split('_').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join('');
    return pascal + 'ed';
  }

  // ── Can-fire check (mirrors go-pflow StateMachine.CanFire) ──

  canFire(places, transitionId) {
    const info = this._transitionMap.get(transitionId);
    if (!info) return false;

    // Check input places have enough tokens
    for (const arc of info.inputs) {
      const w = arc.getWeightSum();
      if ((places[arc.source] || 0) < w) return false;
    }

    // Check inhibitor arcs
    for (const arc of info.inhibitors) {
      if ((places[arc.source] || 0) > 0) return false;
    }

    return true;
  }

  /** Get all enabled transitions for given places */
  enabledTransitions(places) {
    const enabled = [];
    for (const [tId] of this.net.transitions) {
      if (this.canFire(places, tId)) enabled.push(tId);
    }
    return enabled;
  }

  /** Register a callback to be called after a successful fire(). */
  onFire(callback) {
    this._onFireCallbacks.push(callback);
    return () => {
      const idx = this._onFireCallbacks.indexOf(callback);
      if (idx >= 0) this._onFireCallbacks.splice(idx, 1);
    };
  }

  // ── Aggregate lifecycle ──

  /** Create a new aggregate instance, returns aggregate_id */
  create(data = {}) {
    const id = uuid();
    // Find source transitions (transitions with no input arcs — pure creation)
    const createTransitions = [];
    for (const [tId, info] of this._transitionMap) {
      if (info.inputs.length === 0) {
        createTransitions.push(tId);
      }
    }

    if (createTransitions.length > 0) {
      // Fire the first source transition (e.g. create_task, create_project)
      const transId = createTransitions[0];
      const eventType = this._transitionToEventType(transId);
      this.store.append(id, [{ type: eventType, data: { ...data, aggregate_id: id } }], 0);
    } else {
      // No source transitions — emit a generic "Created" event that just establishes the instance
      // The initial token distribution comes from the model definition
      this.store.append(id, [{ type: 'Created', data: { ...data, aggregate_id: id } }], 0);
    }
    return id;
  }

  /** Get the current state of an aggregate */
  getState(id) {
    const state = this._replayState(id);
    const enabled = this.enabledTransitions(state.places);
    return {
      aggregate_id: id,
      version: state.version,
      created_at: state.created_at,
      state: { ...state.data, ...state.places },
      places: state.places,
      enabled_transitions: enabled,
    };
  }

  /** Fire a transition on an aggregate */
  fire(id, transitionId, data = {}) {
    const state = this._replayState(id);

    if (!this.canFire(state.places, transitionId)) {
      throw new Error(`transition '${transitionId}' is not enabled`);
    }

    // Snapshot marking before state change (for provenance/seal verification)
    const prevMarking = { ...state.places };

    const eventType = this._transitionToEventType(transitionId);
    this.store.append(id, [{ type: eventType, data: { ...data, aggregate_id: id, prevMarking } }], state.version);

    const result = this.getState(id);

    // Notify composition hooks
    for (const cb of this._onFireCallbacks) {
      try { cb(transitionId, id, result, prevMarking); } catch (_) { /* ignore */ }
    }

    return result;
  }

  /** Execute a transition by name, auto-detecting aggregate */
  execute(id, transitionId, data = {}) {
    return this.fire(id, transitionId, data);
  }

  // ── Query helpers (for mock API) ──

  /** List all instances with their current state */
  listInstances(limit = 50) {
    const ids = this.store.listInstances();
    const instances = [];
    for (const id of ids.slice(0, limit)) {
      const state = this._replayState(id);
      instances.push({
        id,
        version: state.version,
        created_at: state.created_at,
        places: state.places,
      });
    }
    return instances;
  }

  /** Get aggregate statistics (counts by place) */
  getStats() {
    const instances = this.listInstances(10000);
    const byPlace = {};
    for (const [label] of this.net.places) byPlace[label] = 0;
    for (const inst of instances) {
      for (const [place, count] of Object.entries(inst.places)) {
        if (count > 0) byPlace[place] = (byPlace[place] || 0) + 1;
      }
    }
    return {
      total_instances: instances.length,
      by_place: byPlace,
    };
  }

  /** Get event history for an instance */
  getEvents(id) {
    return this.store.read(id);
  }

  /** Get the schema as the widgets expect it */
  getSchema() {
    const netData = this.modelJson.net || this.modelJson;

    // Build the schema in the format existing widgets expect
    const schema = {
      name: this.name,
      description: this.description,
      places: [],
      transitions: [],
      arcs: [],
    };

    if (netData.places) {
      if (Array.isArray(netData.places)) {
        schema.places = netData.places;
      } else {
        for (const [id, p] of Object.entries(netData.places)) {
          schema.places.push({ id, ...p });
        }
      }
    }

    if (netData.transitions) {
      if (Array.isArray(netData.transitions)) {
        schema.transitions = netData.transitions;
      } else {
        for (const [id, t] of Object.entries(netData.transitions)) {
          schema.transitions.push({ id, ...t });
        }
      }
    }

    schema.arcs = netData.arcs || [];
    return schema;
  }

  /** Get views extension */
  getViews() {
    return this.views;
  }

  /** Get navigation config */
  getNavigation() {
    if (this.pages && this.pages.navigation) return this.pages.navigation;
    return { brand: this.name, items: [] };
  }

  // ── ODE mode ──

  /** Run ODE simulation. Returns ODESolution. */
  ode(customRates = null, tspan = [0, 10], customState = null, solverOptions = {}) {
    // Built from rawNet, not the unfolded net, so ODEProblem does the
    // unfolding itself and the returned ODESolution carries the colorMap that
    // lets getFinalState/getVariable answer in the caller's own place names.
    // customState is therefore in base names too, matching setState.
    const initial = setState(this.rawNet, customState);
    const rates = setRates(this.rawNet, customRates);
    const prob = new ODEProblem(this.rawNet, initial, tspan, rates);
    return solve(prob, Tsit5(), solverOptions);
  }
}

// ============================================================================
// Default export
// ============================================================================

export default {
  Place, Transition, Arc, PetriNet,
  fromJSON, fromModelJSON, loadModel,
  setState, setRates,
  ODEProblem, ODESolution, Tsit5, solve,
  SVGPlotter,
  MemoryEventStore,
  PflowEngine,
};
