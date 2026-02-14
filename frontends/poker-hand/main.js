// Poker Hand Comparison — main.js
import { evaluateHand, compareHands, HandRankNames, parseCards } from './hand-evaluator.js'
import { renderCard } from './cards.js'

// ── State ──
let hand1Cards = []
let hand2Cards = []
let activeHand = 1

const RANKS = ['A','K','Q','J','T','9','8','7','6','5','4','3','2']
const SUITS = [
  { key: 's', name: 'Spades',   symbol: '\u2660', color: 'black' },
  { key: 'h', name: 'Hearts',   symbol: '\u2665', color: 'red'   },
  { key: 'd', name: 'Diamonds', symbol: '\u2666', color: 'red'   },
  { key: 'c', name: 'Clubs',    symbol: '\u2663', color: 'black' },
]

// ── Pre-canned matchups ──
const MATCHUPS = [
  {
    name: 'Royal Flush vs Four of a Kind',
    hand1: ['Ah','Kh','Qh','Jh','Th'],
    hand2: ['9s','9d','9c','9h','Ks'],
  },
  {
    name: 'Straight Flush vs Full House',
    hand1: ['5h','6h','7h','8h','9h'],
    hand2: ['Ks','Kd','Kc','Jh','Jd'],
  },
  {
    name: 'Flush vs Straight',
    hand1: ['2d','5d','8d','Jd','Ad'],
    hand2: ['5s','6h','7c','8d','9s'],
  },
  {
    name: 'Set vs Two Pair',
    hand1: ['9s','9h','9d','3c','7h'],
    hand2: ['Ks','Kd','Qh','Qc','4s'],
  },
  {
    name: 'Overpair vs Top Pair',
    hand1: ['Qs','Qh','7c','8d','2s'],
    hand2: ['Jh','Jd','Ac','8s','3d'],
  },
  {
    name: 'Nut Flush Draw (4 cards)',
    hand1: ['Ah','Kh','7h','2h'],
    hand2: ['Ts','Td','5c','8s'],
  },
  {
    name: 'Broadway Straight vs Low Set',
    hand1: ['As','Kd','Qh','Jc','Ts'],
    hand2: ['3s','3h','3d','7c','9s'],
  },
  {
    name: 'Pocket Aces vs Pocket Kings',
    hand1: ['As','Ah'],
    hand2: ['Ks','Kh'],
  },
  {
    name: 'Full House vs Flush',
    hand1: ['Ts','Th','Td','4s','4h'],
    hand2: ['Ks','Qs','9s','7s','3s'],
  },
  {
    name: 'Identical Straights (Suit Tiebreak)',
    hand1: ['6s','7h','8d','9c','Ts'],
    hand2: ['6h','7d','8c','9s','Td'],
  },
  {
    name: 'High Card Kicker Battle',
    hand1: ['Ah','Kd','Qs','Jc','9h'],
    hand2: ['As','Kc','Qh','Jd','8s'],
  },
]

// ── Render Functions ──

function getActiveCards() {
  return activeHand === 1 ? hand1Cards : hand2Cards
}

function setActiveCards(cards) {
  if (activeHand === 1) hand1Cards = cards
  else hand2Cards = cards
}

function allUsedCards() {
  return new Set([...hand1Cards, ...hand2Cards])
}

function strengthColor(strength) {
  if (strength >= 0.7) return '#2ecc71'
  if (strength >= 0.4) return '#f1c40f'
  if (strength >= 0.1) return '#e67e22'
  return '#e74c3c'
}

function renderHandCards(cards) {
  if (cards.length === 0) {
    let slots = ''
    for (let i = 0; i < 5; i++) {
      slots += '<div class="card-slot">?</div>'
    }
    return slots
  }
  return cards.map(c => renderCard(c)).join('')
}

const RANK_KICKER_WEIGHTS = {
  'A': 4096, 'K': 2048, 'Q': 1024, 'J': 512, 'T': 256,
  '9': 128, '8': 64, '7': 32, '6': 16, '5': 8, '4': 4, '3': 2, '2': 1
}

const SUIT_VALUES = { 's': 3, 'h': 2, 'd': 1, 'c': 0 }
const SUIT_SYMBOLS = { 's': '\u2660', 'h': '\u2665', 'd': '\u2666', 'c': '\u2663' }

function kickerBreakdown(cards) {
  const parsed = parseCards(cards)
  const terms = []
  for (const c of parsed) {
    const rankW = RANK_KICKER_WEIGHTS[c.rank] || 0
    const suitV = SUIT_VALUES[c.suit] || 0
    const weight = rankW * 4 + suitV
    terms.push({ rank: c.rank, suit: c.suit, weight })
  }
  terms.sort((a, b) => b.weight - a.weight)
  return terms.map(t => `${t.rank}${SUIT_SYMBOLS[t.suit]}=${t.weight}`).join(' + ')
}

function renderHandEval(cards) {
  if (cards.length === 0) {
    return '<div class="hand-eval"><div class="hand-rank-name" style="color:rgba(255,255,255,0.3)">Select cards below</div></div>'
  }
  const ev = evaluateHand(cards)
  const color = strengthColor(ev.strength)
  const breakdown = kickerBreakdown(cards)
  return `
    <div class="hand-eval">
      <div class="hand-rank-name">${ev.rankName === 'No Cards' ? 'Evaluating...' : ev.description}</div>
      <div class="hand-description">${ev.rankName} &mdash; Strength: ${(ev.strength * 100).toFixed(1)}%</div>
      <div class="strength-bar">
        <div class="strength-fill" style="width:${ev.strength * 100}%;background:${color}"></div>
      </div>
      <div class="kicker-score">
        Kicker Score: <span class="kicker-value">${ev.petriKickerScore}</span>
        <div class="kicker-breakdown">${breakdown}</div>
      </div>
    </div>
  `
}

function getResult() {
  if (hand1Cards.length === 0 || hand2Cards.length === 0) return null
  const cmp = compareHands(hand1Cards, hand2Cards)
  const ev1 = evaluateHand(hand1Cards)
  const ev2 = evaluateHand(hand2Cards)
  return { cmp, ev1, ev2 }
}

function renderResultBanner() {
  const result = getResult()
  const el = document.getElementById('result-banner')
  if (!result) {
    el.className = 'result-banner hidden'
    el.innerHTML = ''
    return
  }

  const sameRank = result.ev1.rank === result.ev2.rank
  const k1 = result.ev1.petriKickerScore
  const k2 = result.ev2.petriKickerScore

  if (result.cmp > 0) {
    el.className = 'result-banner hand1-wins'
    const kickerDetail = sameRank
      ? ` <span class="kicker-detail">(kicker ${k1} vs ${k2})</span>`
      : ''
    el.innerHTML = `Hand 1 wins! ${result.ev1.description} beats ${result.ev2.description}${kickerDetail}`
  } else if (result.cmp < 0) {
    el.className = 'result-banner hand2-wins'
    const kickerDetail = sameRank
      ? ` <span class="kicker-detail">(kicker ${k2} vs ${k1})</span>`
      : ''
    el.innerHTML = `Hand 2 wins! ${result.ev2.description} beats ${result.ev1.description}${kickerDetail}`
  } else {
    el.className = 'result-banner tie'
    el.innerHTML = `Tie! Both hands: ${result.ev1.description} <span class="kicker-detail">(kicker ${k1} = ${k2})</span>`
  }
}

function updatePanelClasses() {
  const result = getResult()
  const p1 = document.getElementById('hand1-panel')
  const p2 = document.getElementById('hand2-panel')

  p1.className = 'hand-panel'
  p2.className = 'hand-panel'

  if (activeHand === 1) p1.classList.add('active')
  else p2.classList.add('active')

  if (result) {
    if (result.cmp > 0) {
      p1.classList.add('winner')
      p2.classList.add('loser')
    } else if (result.cmp < 0) {
      p2.classList.add('winner')
      p1.classList.add('loser')
    } else {
      p1.classList.add('tied')
      p2.classList.add('tied')
    }
  }
}

function renderPicker() {
  const used = allUsedCards()
  const container = document.getElementById('card-grid')
  let html = ''

  for (const suit of SUITS) {
    html += `<div class="suit-label">${suit.symbol} ${suit.name}</div>`
    for (const rank of RANKS) {
      const cardStr = rank + suit.key
      const isInHand1 = hand1Cards.includes(cardStr)
      const isInHand2 = hand2Cards.includes(cardStr)
      const isSelected = isInHand1 || isInHand2
      const isUsedByOther = (activeHand === 1 && isInHand2) || (activeHand === 2 && isInHand1)

      let cls = `picker-card ${suit.color}`
      if (isInHand1) cls += ' selected hand1'
      else if (isInHand2) cls += ' selected hand2'
      if (isUsedByOther) cls += ' used'

      const displayRank = rank === 'T' ? '10' : rank
      html += `<div class="${cls}" data-card="${cardStr}"><span class="rank">${displayRank}</span><span class="suit">${suit.symbol}</span></div>`
    }
  }
  container.innerHTML = html
  document.getElementById('picker-editing').textContent = `Editing: Hand ${activeHand}`
}

function refresh() {
  document.getElementById('hand1-cards').innerHTML = renderHandCards(hand1Cards)
  document.getElementById('hand2-cards').innerHTML = renderHandCards(hand2Cards)
  document.getElementById('hand1-eval').innerHTML = renderHandEval(hand1Cards)
  document.getElementById('hand2-eval').innerHTML = renderHandEval(hand2Cards)

  // Update hand label highlights
  document.getElementById('hand1-label').className = 'hand-label' + (activeHand === 1 ? ' active' : '')
  document.getElementById('hand2-label').className = 'hand-label' + (activeHand === 2 ? ' active' : '')

  renderPicker()
  renderResultBanner()
  updatePanelClasses()
}

// ── Event Handlers ──

function onPickerClick(e) {
  const el = e.target.closest('.picker-card')
  if (!el || el.classList.contains('used')) return

  const card = el.dataset.card
  const cards = getActiveCards()

  const idx = cards.indexOf(card)
  if (idx >= 0) {
    cards.splice(idx, 1)
  } else {
    if (cards.length >= 7) return // max 7 cards
    cards.push(card)
  }

  setActiveCards(cards)
  refresh()
}

function onClearHand(handNum) {
  if (handNum === 1) hand1Cards = []
  else hand2Cards = []
  refresh()
}

function onSelectHand(handNum) {
  activeHand = handNum
  refresh()
}

function onLoadMatchup(matchup) {
  hand1Cards = [...matchup.hand1]
  hand2Cards = [...matchup.hand2]
  activeHand = 1
  refresh()
}

function onCardInHandClick(e, handNum) {
  const cardEl = e.target.closest('.card')
  if (!cardEl) return

  // Find which card was clicked by index
  const cards = handNum === 1 ? hand1Cards : hand2Cards
  const cardEls = e.currentTarget.querySelectorAll('.card')
  const idx = Array.from(cardEls).indexOf(cardEl)
  if (idx >= 0 && idx < cards.length) {
    cards.splice(idx, 1)
    if (handNum === 1) hand1Cards = cards
    else hand2Cards = cards
    refresh()
  }
}

// ── Initialize ──

export function init() {
  const app = document.getElementById('app')
  app.innerHTML = `
    <h1>Poker Hand Comparison</h1>
    <p class="subtitle">Build two hands and see which one wins</p>

    <div id="result-banner" class="result-banner hidden"></div>

    <div class="hands-area">
      <div class="hand-panel active" id="hand1-panel">
        <div class="hand-header">
          <span class="hand-label active" id="hand1-label">Hand 1</span>
          <button class="hand-clear" id="clear1">Clear</button>
        </div>
        <div class="hand-cards" id="hand1-cards"></div>
        <div id="hand1-eval"></div>
      </div>

      <div class="vs-divider">VS</div>

      <div class="hand-panel" id="hand2-panel">
        <div class="hand-header">
          <span class="hand-label" id="hand2-label">Hand 2</span>
          <button class="hand-clear" id="clear2">Clear</button>
        </div>
        <div class="hand-cards" id="hand2-cards"></div>
        <div id="hand2-eval"></div>
      </div>
    </div>

    <div class="card-picker">
      <div class="picker-header">
        <span class="picker-title">Click cards to add/remove</span>
        <span class="picker-editing" id="picker-editing">Editing: Hand 1</span>
      </div>
      <div class="card-grid" id="card-grid"></div>
    </div>

    <div class="matchups-section">
      <div class="matchups-title">Try a classic matchup:</div>
      <div class="matchup-chips" id="matchup-chips"></div>
    </div>

    <div class="explainer-section">
      <pflow-concepts service="poker-hand" concepts="places,transitions,arcs,events"></pflow-concepts>

      <explainer-panel title="Classification Net" icon="🎯">
        <p>The poker-hand model is a <strong>ClassificationNet</strong> &mdash; a Petri net designed purely for pattern detection. Unlike workflow nets that model processes, this net takes a set of cards as input tokens and determines which poker hand rank they form.</p>
      </explainer-panel>

      <explainer-panel title="Places as Card Patterns" icon="📍">
        <p>Each place in the net represents a card grouping: <em>rank groups</em> (how many cards share a rank) and <em>suit groups</em> (how many cards share a suit). Tokens in these places encode the hand's structure without caring about specific card values.</p>
      </explainer-panel>

      <explainer-panel title="Transitions as Pattern Recognition" icon="⚡">
        <p>Transitions fire when input places have the right token configuration. For example, the &ldquo;full_house&rdquo; transition requires 3 tokens in a rank-group place AND 2 tokens in another &mdash; exactly the definition of a full house. The net is the classifier.</p>
      </explainer-panel>

      <explainer-panel title="Arcs and Token Flow" icon="➡️">
        <p>Arcs with weights connect card-pattern places to classification transitions. A flush needs 5 tokens from one suit place (weight=5 arc). The Petri net formalism makes the classification rules explicit, visual, and formally verifiable.</p>
      </explainer-panel>

      <explainer-panel title="Kicker Scoring with Weighted Arcs" icon="🏆">
        <p>When two hands share the same rank (e.g. both &ldquo;High Card A&rdquo;), the <strong>kicker score</strong> breaks the tie. The Petri net encodes this using <strong>power-of-2 weighted arcs</strong>:</p>
        <p>Each of the 52 cards has a detection transition (<code>hc_*</code>) that fires when that card is in hand. The output arc to <code>kicker_score</code> carries a weight of <strong>2<sup>n</sup></strong> based on rank: A&rarr;4096, K&rarr;2048, Q&rarr;1024, &hellip; 2&rarr;1.</p>
        <p>This binary encoding guarantees that any single higher-rank card outweighs all lower cards combined (2<sup>n</sup> &gt; 2<sup>n&minus;1</sup> + &hellip; + 2<sup>0</sup>), matching the lexicographic comparison poker uses. Try the <strong>High Card Kicker Battle</strong> matchup to see it in action &mdash; the 9-kicker (128) vs 8-kicker (64) is the difference between 7808 and 7744.</p>
      </explainer-panel>

      <pflow-model-link model="poker-hand"></pflow-model-link>
    </div>
  `

  // Render matchup chips
  const chipsEl = document.getElementById('matchup-chips')
  chipsEl.innerHTML = MATCHUPS.map((m, i) =>
    `<button class="matchup-chip" data-idx="${i}">${m.name}</button>`
  ).join('')

  // Bind events
  document.getElementById('card-grid').addEventListener('click', onPickerClick)
  document.getElementById('clear1').addEventListener('click', () => onClearHand(1))
  document.getElementById('clear2').addEventListener('click', () => onClearHand(2))
  document.getElementById('hand1-label').addEventListener('click', () => onSelectHand(1))
  document.getElementById('hand2-label').addEventListener('click', () => onSelectHand(2))
  document.getElementById('hand1-cards').addEventListener('click', e => onCardInHandClick(e, 1))
  document.getElementById('hand2-cards').addEventListener('click', e => onCardInHandClick(e, 2))

  chipsEl.addEventListener('click', e => {
    const chip = e.target.closest('.matchup-chip')
    if (!chip) return
    onLoadMatchup(MATCHUPS[parseInt(chip.dataset.idx)])
  })

  refresh()
}
