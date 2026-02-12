/**
 * <time-control> Web Component — playback controls for ODE simulation
 *
 * Play/pause, step forward, speed selector, time slider, wall clock display.
 * Dispatches 'time-change' events with { time, playing }.
 */

class TimeControl extends HTMLElement {
    constructor() {
        super();
        this._playing = false;
        this._time = 0;        // hours into the day (0-10)
        this._speed = 1;       // multiplier
        this._maxTime = 10;    // 10-hour day: 8 AM - 6 PM
        this._animFrame = null;
        this._lastTick = null;
    }

    connectedCallback() {
        this.render();
        this._bindEvents();
    }

    disconnectedCallback() {
        this.pause();
    }

    get time() { return this._time; }
    set time(v) {
        this._time = Math.max(0, Math.min(this._maxTime, v));
        this._updateDisplay();
        this.dispatchEvent(new CustomEvent('time-change', { detail: { time: this._time, playing: this._playing }, bubbles: true }));
    }

    render() {
        this.innerHTML = `
        <div class="time-control">
            <div class="clock-display" id="tc-clock">8:00 AM</div>
            <input type="range" class="time-slider" id="tc-slider" min="0" max="${this._maxTime}" step="0.01" value="0">
            <div class="transport">
                <button id="tc-reset" title="Reset">&#x23EE;</button>
                <button id="tc-play" title="Play/Pause">&#x25B6;</button>
                <button id="tc-step" title="Step +5min">&#x23ED;</button>
                <select class="speed-select" id="tc-speed">
                    <option value="0.5">0.5x</option>
                    <option value="1" selected>1x</option>
                    <option value="2">2x</option>
                    <option value="5">5x</option>
                    <option value="10">10x</option>
                </select>
            </div>
        </div>`;
    }

    _bindEvents() {
        this.querySelector('#tc-play').addEventListener('click', () => this.togglePlay());
        this.querySelector('#tc-step').addEventListener('click', () => this.step());
        this.querySelector('#tc-reset').addEventListener('click', () => this.reset());
        this.querySelector('#tc-slider').addEventListener('input', (e) => {
            this.pause();
            this.time = parseFloat(e.target.value);
        });
        this.querySelector('#tc-speed').addEventListener('change', (e) => {
            this._speed = parseFloat(e.target.value);
        });
    }

    _updateDisplay() {
        const clock = this.querySelector('#tc-clock');
        const slider = this.querySelector('#tc-slider');
        if (clock) {
            const totalMinutes = Math.floor(this._time * 60);
            const h = 8 + Math.floor(totalMinutes / 60);
            const m = totalMinutes % 60;
            const ampm = h >= 12 ? 'PM' : 'AM';
            const h12 = h > 12 ? h - 12 : h;
            clock.textContent = `${h12}:${m.toString().padStart(2,'0')} ${ampm}`;
        }
        if (slider) slider.value = this._time;

        const playBtn = this.querySelector('#tc-play');
        if (playBtn) {
            playBtn.innerHTML = this._playing ? '&#x23F8;' : '&#x25B6;';
            playBtn.classList.toggle('active', this._playing);
        }
    }

    togglePlay() {
        if (this._playing) {
            this.pause();
        } else {
            this.play();
        }
    }

    play() {
        if (this._time >= this._maxTime) this._time = 0;
        this._playing = true;
        this._lastTick = performance.now();
        this._updateDisplay();
        this._tick();
    }

    pause() {
        this._playing = false;
        if (this._animFrame) cancelAnimationFrame(this._animFrame);
        this._animFrame = null;
        this._updateDisplay();
    }

    step() {
        this.pause();
        this.time = this._time + 1/12; // +5 minutes
    }

    reset() {
        this.pause();
        this.time = 0;
    }

    _tick() {
        if (!this._playing) return;
        const now = performance.now();
        const dt = (now - this._lastTick) / 1000; // seconds elapsed
        this._lastTick = now;

        // Advance simulation time: real-time * speed
        // 1x speed = 1 sim-hour per 60 real-seconds (i.e., 8-hour day in 8 minutes)
        const simDt = dt * this._speed / 60;
        this._time += simDt;

        if (this._time >= this._maxTime) {
            this._time = this._maxTime;
            this.pause();
        }

        this._updateDisplay();
        this.dispatchEvent(new CustomEvent('time-change', { detail: { time: this._time, playing: this._playing }, bubbles: true }));

        if (this._playing) {
            this._animFrame = requestAnimationFrame(() => this._tick());
        }
    }
}

customElements.define('time-control', TimeControl);
export { TimeControl };
