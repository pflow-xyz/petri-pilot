// Tests for hand-evaluator.js
// Run with: node --experimental-vm-modules hand-evaluator.test.js
// Or in browser console after importing

import {
  HandRank,
  HandRankNames,
  PetriNetStrength,
  parseCard,
  parseCards,
  evaluateHand,
  evaluatePokerHand,
  compareHands,
  getPreflopStrength,
  calculateDraws
} from './hand-evaluator.js'

let passed = 0
let failed = 0

function assert(condition, message) {
  if (condition) {
    passed++
    console.log(`✓ ${message}`)
  } else {
    failed++
    console.error(`✗ ${message}`)
  }
}

function assertEqual(actual, expected, message) {
  if (actual === expected) {
    passed++
    console.log(`✓ ${message}`)
  } else {
    failed++
    console.error(`✗ ${message}: expected ${expected}, got ${actual}`)
  }
}

// Test hand classification (matching Go test cases)
console.log('\n=== Hand Classification Tests ===')

const classificationTests = [
  // Basic hands
  { name: 'High card', hole: 'Ah,Kd', community: 'Qs,Jc,9h', expected: HandRank.HIGH_CARD },
  { name: 'Pair of Aces', hole: 'Ah,Ad', community: '', expected: HandRank.PAIR },
  { name: 'Pair with board', hole: 'Ah,Kd', community: 'As,Jc,9h', expected: HandRank.PAIR },
  { name: 'Two pair', hole: 'Ah,Kh', community: 'Ad,Kd,9s', expected: HandRank.TWO_PAIR },
  { name: 'Three of a kind', hole: 'Ah,Ad', community: 'As,Kd,Qc', expected: HandRank.THREE_OF_A_KIND },
  { name: 'Full house', hole: 'Ah,Ad', community: 'As,Kd,Kc', expected: HandRank.FULL_HOUSE },
  { name: 'Four of a kind', hole: 'Ah,Ad', community: 'As,Ac,Kd', expected: HandRank.FOUR_OF_A_KIND },

  // Straights
  { name: 'Broadway straight', hole: 'Ah,Kd', community: 'Qs,Jc,Th', expected: HandRank.STRAIGHT },
  { name: 'Wheel straight', hole: 'Ah,2d', community: '3s,4c,5h', expected: HandRank.STRAIGHT },
  { name: 'Middle straight', hole: '9h,8d', community: '7s,6c,5h', expected: HandRank.STRAIGHT },

  // Flushes
  { name: 'Ace-high flush', hole: 'Ah,Kh', community: 'Qh,Jh,9h', expected: HandRank.FLUSH },
  { name: 'Low flush', hole: '7h,6h', community: '5h,4h,2h', expected: HandRank.FLUSH },

  // Straight flush
  { name: 'Royal flush', hole: 'Ah,Kh', community: 'Qh,Jh,Th', expected: HandRank.STRAIGHT_FLUSH },
  { name: 'Steel wheel', hole: 'Ah,2h', community: '3h,4h,5h', expected: HandRank.STRAIGHT_FLUSH },
  { name: 'Middle straight flush', hole: '9h,8h', community: '7h,6h,5h', expected: HandRank.STRAIGHT_FLUSH },
]

for (const tc of classificationTests) {
  const cards = tc.hole + (tc.community ? ',' + tc.community : '')
  const result = evaluateHand(cards.split(',').filter(c => c))
  assertEqual(result.rank, tc.expected, `${tc.name}: ${HandRankNames[result.rank]}`)
}

// Test ranking order
console.log('\n=== Ranking Order Tests ===')

const rankingTests = [
  { hole: 'Ah,Kd', community: 'Qs,Jc,9h,7s,3c', expected: HandRank.HIGH_CARD },
  { hole: 'Ah,Ad', community: 'Qs,Jc,9h', expected: HandRank.PAIR },
  { hole: 'Ah,Kh', community: 'Ad,Kd,9s', expected: HandRank.TWO_PAIR },
  { hole: 'Ah,Ad', community: 'As,Kd,Qc', expected: HandRank.THREE_OF_A_KIND },
  { hole: 'Ah,Kd', community: 'Qs,Jc,Th', expected: HandRank.STRAIGHT },
  { hole: 'Ah,Kh', community: 'Qh,Jh,9h', expected: HandRank.FLUSH },
  { hole: 'Ah,Ad', community: 'As,Kd,Kc', expected: HandRank.FULL_HOUSE },
  { hole: 'Ah,Ad', community: 'As,Ac,Kd', expected: HandRank.FOUR_OF_A_KIND },
  { hole: 'Ah,Kh', community: 'Qh,Jh,Th', expected: HandRank.STRAIGHT_FLUSH },
]

let prevStrength = -1
for (const tc of rankingTests) {
  const cards = tc.hole + ',' + tc.community
  const result = evaluateHand(cards.split(','))

  assert(result.strength > prevStrength,
    `${HandRankNames[tc.expected]} (${result.strength.toFixed(3)}) > previous (${prevStrength.toFixed(3)})`)

  prevStrength = result.strength
}

// Test Petri net strength values
console.log('\n=== Petri Net Strength Tests ===')

const petriTests = [
  { rank: HandRank.HIGH_CARD, expected: 0 },
  { rank: HandRank.PAIR, expected: 2 },
  { rank: HandRank.TWO_PAIR, expected: 3 },
  { rank: HandRank.THREE_OF_A_KIND, expected: 4 },
  { rank: HandRank.STRAIGHT, expected: 5 },
  { rank: HandRank.FLUSH, expected: 6 },
  { rank: HandRank.FULL_HOUSE, expected: 7 },
  { rank: HandRank.FOUR_OF_A_KIND, expected: 8 },
  { rank: HandRank.STRAIGHT_FLUSH, expected: 9 },
]

for (const tc of petriTests) {
  assertEqual(PetriNetStrength[tc.rank], tc.expected,
    `${HandRankNames[tc.rank]} petri strength = ${tc.expected}`)
}

// Test partial hands (preflop, flop, etc.)
console.log('\n=== Partial Hand Tests ===')

const partialTests = [
  { name: 'Pocket Aces (2 cards)', hole: 'Ah,Ad', community: '', expected: HandRank.PAIR },
  { name: 'AK suited (2 cards)', hole: 'Ah,Kh', community: '', expected: HandRank.HIGH_CARD },
  { name: 'Flopped set (3 cards)', hole: 'Ah,Ad', community: 'As', expected: HandRank.THREE_OF_A_KIND },
  { name: 'Two pair (4 cards)', hole: 'Ah,Kh', community: 'Ad,Kd', expected: HandRank.TWO_PAIR },
  { name: 'Quads (4 cards)', hole: 'Ah,Ad', community: 'As,Ac', expected: HandRank.FOUR_OF_A_KIND },
]

for (const tc of partialTests) {
  const allCards = [tc.hole, tc.community].filter(c => c).join(',').split(',').filter(c => c)
  const result = evaluateHand(allCards)
  assertEqual(result.rank, tc.expected, `${tc.name}: ${HandRankNames[result.rank]}`)
}

// Test hand comparison
console.log('\n=== Hand Comparison Tests ===')

assert(compareHands('Ah,Ad,As,Kd,Qc'.split(','), 'Kh,Kd,Ks,Qc,Jc'.split(',')) > 0,
  'Trip Aces beats Trip Kings')

assert(compareHands('Ah,Kh,Qh,Jh,Th'.split(','), 'Ah,Ad,As,Ac,Kd'.split(',')) > 0,
  'Royal Flush beats Quad Aces')

assert(compareHands('Ah,Kd,Qs,Jc,Th'.split(','), '9h,8d,7s,6c,5h'.split(',')) > 0,
  'Broadway straight beats 9-high straight')

assertEqual(compareHands('Ah,Kd,Qs,Jc,9h'.split(','), 'Ah,Kd,Qs,Jc,9h'.split(',')), 0,
  'Identical hands tie')

// Test preflop strength
console.log('\n=== Preflop Strength Tests ===')

const preflopTests = [
  { hole: 'Ah,Ad', desc: 'Pocket Aces', minStrength: 0.6 },
  { hole: 'Kh,Kd', desc: 'Pocket Kings', minStrength: 0.55 },
  { hole: 'Ah,Kh', desc: 'AK suited', minStrength: 0.4 },
  { hole: 'Ah,Kd', desc: 'AK offsuit', minStrength: 0.35 },
  { hole: '7h,2d', desc: '72 offsuit', maxStrength: 0.25 },
]

for (const tc of preflopTests) {
  const result = getPreflopStrength(tc.hole.split(','))
  if (tc.minStrength !== undefined) {
    assert(result.strength >= tc.minStrength,
      `${tc.desc} strength ${result.strength.toFixed(2)} >= ${tc.minStrength}`)
  }
  if (tc.maxStrength !== undefined) {
    assert(result.strength <= tc.maxStrength,
      `${tc.desc} strength ${result.strength.toFixed(2)} <= ${tc.maxStrength}`)
  }
}

// Test drawing calculations
console.log('\n=== Draw Calculation Tests ===')

const drawTests = [
  { name: 'Flush draw', hole: 'Ah,Kh', community: { flop: ['Qh', 'Jh', '2s'] }, expectFlush: 9 },
  { name: 'No draw', hole: 'Ah,Kd', community: { flop: ['2s', '7c', 'Jh'] }, expectFlush: 0 },
]

for (const tc of drawTests) {
  const result = calculateDraws(tc.hole.split(','), tc.community)
  assertEqual(result.flushDraw, tc.expectFlush, `${tc.name}: flush draw = ${tc.expectFlush} outs`)
}

// Test evaluatePokerHand helper
console.log('\n=== evaluatePokerHand Tests ===')

const evalTest = evaluatePokerHand(['Ah', 'Ad'], { flop: ['As', 'Kd', 'Kc'] })
assertEqual(evalTest.rank, HandRank.FULL_HOUSE, 'evaluatePokerHand finds full house')

// Summary
console.log('\n=== Summary ===')
console.log(`Passed: ${passed}`)
console.log(`Failed: ${failed}`)
console.log(failed === 0 ? '✓ All tests passed!' : '✗ Some tests failed')

// Export for browser use
if (typeof window !== 'undefined') {
  window.handEvaluatorTests = { passed, failed }
}
