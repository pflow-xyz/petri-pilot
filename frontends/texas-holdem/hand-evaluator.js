// Poker Hand Evaluator for Texas Hold'em
// Matches the standard hand ranking algorithm from pkg/serve/poker_hand_test.go

/**
 * Hand rank constants (higher = better)
 */
export const HandRank = {
  HIGH_CARD: 0,
  PAIR: 1,
  TWO_PAIR: 2,
  THREE_OF_A_KIND: 3,
  STRAIGHT: 4,
  FLUSH: 5,
  FULL_HOUSE: 6,
  FOUR_OF_A_KIND: 7,
  STRAIGHT_FLUSH: 8
}

/**
 * Hand rank names for display
 */
export const HandRankNames = {
  [HandRank.HIGH_CARD]: 'High Card',
  [HandRank.PAIR]: 'Pair',
  [HandRank.TWO_PAIR]: 'Two Pair',
  [HandRank.THREE_OF_A_KIND]: 'Three of a Kind',
  [HandRank.STRAIGHT]: 'Straight',
  [HandRank.FLUSH]: 'Flush',
  [HandRank.FULL_HOUSE]: 'Full House',
  [HandRank.FOUR_OF_A_KIND]: 'Four of a Kind',
  [HandRank.STRAIGHT_FLUSH]: 'Straight Flush'
}

/**
 * Petri net strength values (matching buildPokerHandModel)
 */
export const PetriNetStrength = {
  [HandRank.HIGH_CARD]: 0,
  [HandRank.PAIR]: 2,
  [HandRank.TWO_PAIR]: 3,
  [HandRank.THREE_OF_A_KIND]: 4,
  [HandRank.STRAIGHT]: 5,
  [HandRank.FLUSH]: 6,
  [HandRank.FULL_HOUSE]: 7,
  [HandRank.FOUR_OF_A_KIND]: 8,
  [HandRank.STRAIGHT_FLUSH]: 9
}

/**
 * Rank values for comparison (A=14, K=13, ..., 2=2)
 */
const RANK_VALUES = {
  'A': 14, 'K': 13, 'Q': 12, 'J': 11, 'T': 10, '10': 10,
  '9': 9, '8': 8, '7': 7, '6': 6, '5': 5, '4': 4, '3': 3, '2': 2
}

/**
 * Straight patterns (high card first)
 */
const STRAIGHT_PATTERNS = [
  ['A', 'K', 'Q', 'J', 'T'],  // Broadway
  ['K', 'Q', 'J', 'T', '9'],
  ['Q', 'J', 'T', '9', '8'],
  ['J', 'T', '9', '8', '7'],
  ['T', '9', '8', '7', '6'],
  ['9', '8', '7', '6', '5'],
  ['8', '7', '6', '5', '4'],
  ['7', '6', '5', '4', '3'],
  ['6', '5', '4', '3', '2'],
  ['5', '4', '3', '2', 'A']   // Wheel (A plays low)
]

/**
 * Parse a card string like "Ah" or "10s" into {rank, suit}
 * @param {string} cardStr - Card string
 * @returns {object|null} - {rank, suit} or null if invalid
 */
export function parseCard(cardStr) {
  if (!cardStr || cardStr.length < 2) return null

  const suit = cardStr.slice(-1).toLowerCase()
  let rank = cardStr.slice(0, -1).toUpperCase()

  // Normalize 10 to T
  if (rank === '10') rank = 'T'

  // Validate
  if (!RANK_VALUES[rank]) return null
  if (!['h', 'd', 'c', 's'].includes(suit)) return null

  return { rank, suit }
}

/**
 * Parse multiple cards from comma-separated string or array
 * @param {string|array} cards - Cards to parse
 * @returns {array} - Array of {rank, suit} objects
 */
export function parseCards(cards) {
  if (!cards) return []

  const cardArray = typeof cards === 'string'
    ? cards.split(',').map(c => c.trim())
    : cards

  return cardArray
    .map(c => typeof c === 'string' ? parseCard(c) : c)
    .filter(c => c !== null)
}

/**
 * Get rank value for comparison
 * @param {string} rank - Card rank
 * @returns {number} - Numeric value (2-14)
 */
export function getRankValue(rank) {
  return RANK_VALUES[rank] || 0
}

/**
 * Evaluate a poker hand and return detailed result
 * @param {array} cards - Array of cards (strings or parsed objects)
 * @returns {object} - {rank, rankName, strength, kickers, description}
 */
export function evaluateHand(cards) {
  const parsed = parseCards(cards)

  if (parsed.length === 0) {
    return {
      rank: HandRank.HIGH_CARD,
      rankName: 'No Cards',
      strength: 0,
      petriStrength: 0,
      kickers: [],
      description: 'No cards'
    }
  }

  // Count ranks and suits
  const rankCounts = {}
  const suitCounts = {}
  const cardsBySuit = { h: [], d: [], c: [], s: [] }

  for (const card of parsed) {
    rankCounts[card.rank] = (rankCounts[card.rank] || 0) + 1
    suitCounts[card.suit] = (suitCounts[card.suit] || 0) + 1
    cardsBySuit[card.suit].push(card)
  }

  // Find flush suit (if any)
  let flushSuit = null
  for (const [suit, count] of Object.entries(suitCounts)) {
    if (count >= 5) {
      flushSuit = suit
      break
    }
  }

  // Check for straight
  const hasStraight = checkStraight(rankCounts)

  // Check for straight flush
  if (flushSuit) {
    const flushRanks = {}
    for (const card of cardsBySuit[flushSuit]) {
      flushRanks[card.rank] = true
    }
    if (checkStraight(flushRanks)) {
      const highCard = getStraightHighCard(flushRanks)
      return {
        rank: HandRank.STRAIGHT_FLUSH,
        rankName: highCard === 'A' ? 'Royal Flush' : 'Straight Flush',
        strength: normalizeStrength(HandRank.STRAIGHT_FLUSH, [getRankValue(highCard)]),
        petriStrength: PetriNetStrength[HandRank.STRAIGHT_FLUSH],
        kickers: [highCard],
        description: highCard === 'A' ? 'Royal Flush' : `Straight Flush, ${highCard}-high`
      }
    }
  }

  // Count pairs, trips, quads
  const counts = Object.entries(rankCounts)
    .map(([rank, count]) => ({ rank, count, value: getRankValue(rank) }))
    .sort((a, b) => b.count - a.count || b.value - a.value)

  const quads = counts.filter(c => c.count === 4)
  const trips = counts.filter(c => c.count === 3)
  const pairs = counts.filter(c => c.count === 2)

  // Four of a kind
  if (quads.length > 0) {
    const kicker = counts.find(c => c.count < 4)
    return {
      rank: HandRank.FOUR_OF_A_KIND,
      rankName: 'Four of a Kind',
      strength: normalizeStrength(HandRank.FOUR_OF_A_KIND, [quads[0].value, kicker?.value || 0]),
      petriStrength: PetriNetStrength[HandRank.FOUR_OF_A_KIND],
      kickers: [quads[0].rank, kicker?.rank].filter(Boolean),
      description: `Four ${quads[0].rank}s`
    }
  }

  // Full house (trips + pair, or two trips)
  if (trips.length > 0 && (pairs.length > 0 || trips.length >= 2)) {
    const tripRank = trips[0]
    const pairRank = trips.length >= 2 ? trips[1] : pairs[0]
    return {
      rank: HandRank.FULL_HOUSE,
      rankName: 'Full House',
      strength: normalizeStrength(HandRank.FULL_HOUSE, [tripRank.value, pairRank.value]),
      petriStrength: PetriNetStrength[HandRank.FULL_HOUSE],
      kickers: [tripRank.rank, pairRank.rank],
      description: `Full House, ${tripRank.rank}s full of ${pairRank.rank}s`
    }
  }

  // Flush
  if (flushSuit) {
    const flushCards = cardsBySuit[flushSuit]
      .map(c => ({ rank: c.rank, value: getRankValue(c.rank) }))
      .sort((a, b) => b.value - a.value)
      .slice(0, 5)
    return {
      rank: HandRank.FLUSH,
      rankName: 'Flush',
      strength: normalizeStrength(HandRank.FLUSH, flushCards.map(c => c.value)),
      petriStrength: PetriNetStrength[HandRank.FLUSH],
      kickers: flushCards.map(c => c.rank),
      description: `Flush, ${flushCards[0].rank}-high`
    }
  }

  // Straight
  if (hasStraight) {
    const highCard = getStraightHighCard(rankCounts)
    return {
      rank: HandRank.STRAIGHT,
      rankName: 'Straight',
      strength: normalizeStrength(HandRank.STRAIGHT, [getRankValue(highCard)]),
      petriStrength: PetriNetStrength[HandRank.STRAIGHT],
      kickers: [highCard],
      description: `Straight, ${highCard}-high`
    }
  }

  // Three of a kind
  if (trips.length > 0) {
    const kickers = counts
      .filter(c => c.count < 3)
      .slice(0, 2)
    return {
      rank: HandRank.THREE_OF_A_KIND,
      rankName: 'Three of a Kind',
      strength: normalizeStrength(HandRank.THREE_OF_A_KIND, [trips[0].value, ...kickers.map(k => k.value)]),
      petriStrength: PetriNetStrength[HandRank.THREE_OF_A_KIND],
      kickers: [trips[0].rank, ...kickers.map(k => k.rank)],
      description: `Three ${trips[0].rank}s`
    }
  }

  // Two pair
  if (pairs.length >= 2) {
    const kicker = counts.find(c => c.count === 1)
    return {
      rank: HandRank.TWO_PAIR,
      rankName: 'Two Pair',
      strength: normalizeStrength(HandRank.TWO_PAIR, [pairs[0].value, pairs[1].value, kicker?.value || 0]),
      petriStrength: PetriNetStrength[HandRank.TWO_PAIR],
      kickers: [pairs[0].rank, pairs[1].rank, kicker?.rank].filter(Boolean),
      description: `Two Pair, ${pairs[0].rank}s and ${pairs[1].rank}s`
    }
  }

  // One pair
  if (pairs.length === 1) {
    const kickers = counts
      .filter(c => c.count === 1)
      .slice(0, 3)
    return {
      rank: HandRank.PAIR,
      rankName: 'Pair',
      strength: normalizeStrength(HandRank.PAIR, [pairs[0].value, ...kickers.map(k => k.value)]),
      petriStrength: PetriNetStrength[HandRank.PAIR],
      kickers: [pairs[0].rank, ...kickers.map(k => k.rank)],
      description: `Pair of ${pairs[0].rank}s`
    }
  }

  // High card
  const highCards = counts.slice(0, 5)
  return {
    rank: HandRank.HIGH_CARD,
    rankName: 'High Card',
    strength: normalizeStrength(HandRank.HIGH_CARD, highCards.map(c => c.value)),
    petriStrength: PetriNetStrength[HandRank.HIGH_CARD],
    kickers: highCards.map(c => c.rank),
    description: `High Card ${highCards[0].rank}`
  }
}

/**
 * Check if ranks contain a straight
 * @param {object} rankCounts - Map of rank -> count
 * @returns {boolean}
 */
function checkStraight(rankCounts) {
  for (const pattern of STRAIGHT_PATTERNS) {
    if (pattern.every(rank => rankCounts[rank] > 0)) {
      return true
    }
  }
  return false
}

/**
 * Get the high card of a straight
 * @param {object} rankCounts - Map of rank -> count
 * @returns {string} - High card rank
 */
function getStraightHighCard(rankCounts) {
  for (const pattern of STRAIGHT_PATTERNS) {
    if (pattern.every(rank => rankCounts[rank] > 0)) {
      // Return high card (first in pattern, except for wheel which is 5-high)
      return pattern[0] === '5' ? '5' : pattern[0]
    }
  }
  return 'A'
}

/**
 * Normalize hand strength to 0-1 scale
 * @param {number} handRank - The hand rank (0-8)
 * @param {array} kickers - Kicker values for tiebreaking
 * @returns {number} - Normalized strength (0-1)
 */
function normalizeStrength(handRank, kickers) {
  // Base strength from hand rank (each rank is worth ~0.1)
  const baseStrength = handRank * 0.1

  // Add kicker value within the rank range
  // Max kicker contribution is 0.09 to stay within rank band
  let kickerBonus = 0
  const maxKickerValue = 14 // Ace

  for (let i = 0; i < kickers.length && i < 5; i++) {
    const weight = Math.pow(0.1, i + 1)
    kickerBonus += (kickers[i] / maxKickerValue) * weight * 0.9
  }

  return Math.min(0.99, baseStrength + kickerBonus)
}

/**
 * Compare two hands and return the winner
 * @param {array} hand1 - First hand cards
 * @param {array} hand2 - Second hand cards
 * @returns {number} - 1 if hand1 wins, -1 if hand2 wins, 0 if tie
 */
export function compareHands(hand1, hand2) {
  const eval1 = evaluateHand(hand1)
  const eval2 = evaluateHand(hand2)

  // Compare by rank first
  if (eval1.rank !== eval2.rank) {
    return eval1.rank > eval2.rank ? 1 : -1
  }

  // Same rank - compare kickers
  const kickers1 = eval1.kickers.map(k => getRankValue(k))
  const kickers2 = eval2.kickers.map(k => getRankValue(k))

  for (let i = 0; i < Math.max(kickers1.length, kickers2.length); i++) {
    const k1 = kickers1[i] || 0
    const k2 = kickers2[i] || 0
    if (k1 !== k2) {
      return k1 > k2 ? 1 : -1
    }
  }

  return 0 // Exact tie
}

/**
 * Evaluate hand strength for auto-play decisions
 * Combines hole cards with community cards
 * @param {array} holeCards - Player's hole cards
 * @param {object} communityCards - {flop: [], turn: [], river: []}
 * @returns {object} - Full evaluation result
 */
export function evaluatePokerHand(holeCards, communityCards = {}) {
  const allCards = [
    ...parseCards(holeCards),
    ...parseCards(communityCards.flop || []),
    ...parseCards(communityCards.turn || []),
    ...parseCards(communityCards.river || [])
  ]

  // Convert back to strings for evaluateHand
  const cardStrings = allCards.map(c => c.rank + c.suit)
  return evaluateHand(cardStrings)
}

/**
 * Get preflop hand strength (hole cards only)
 * @param {array} holeCards - Player's two hole cards
 * @returns {object} - {strength, category, description}
 */
export function getPreflopStrength(holeCards) {
  const parsed = parseCards(holeCards)
  if (parsed.length < 2) {
    return { strength: 0, category: 'unknown', description: 'Need 2 cards' }
  }

  const [card1, card2] = parsed
  const val1 = getRankValue(card1.rank)
  const val2 = getRankValue(card2.rank)
  const highVal = Math.max(val1, val2)
  const lowVal = Math.min(val1, val2)
  const isPair = card1.rank === card2.rank
  const isSuited = card1.suit === card2.suit
  const gap = highVal - lowVal
  const isConnected = gap === 1

  // Base strength from high card (0-0.3)
  let strength = (highVal / 14) * 0.3

  // Pair bonus (0.2-0.4 depending on pair rank)
  if (isPair) {
    strength += 0.2 + (highVal / 14) * 0.2
  }

  // Suited bonus
  if (isSuited) {
    strength += 0.05
  }

  // Connected bonus
  if (isConnected) {
    strength += 0.03
  } else if (gap <= 3) {
    strength += 0.01 // One-gapper, two-gapper
  }

  // Premium hands adjustment
  if (isPair && highVal >= 10) {
    strength += 0.1 // Premium pairs (TT+)
  }
  if (!isPair && highVal === 14 && lowVal >= 10) {
    strength += 0.08 // Big ace (AT+)
  }

  // Categorize
  let category = 'speculative'
  if (strength > 0.6) category = 'premium'
  else if (strength > 0.45) category = 'strong'
  else if (strength > 0.35) category = 'playable'

  // Description
  const suitedStr = isSuited ? 's' : 'o'
  const pairStr = isPair ? `Pocket ${card1.rank}s` : `${card1.rank}${card2.rank}${suitedStr}`

  return {
    strength: Math.min(1, strength),
    category,
    description: pairStr,
    isPair,
    isSuited,
    isConnected,
    gap
  }
}

/**
 * Calculate drawing potential (outs and equity)
 * @param {array} holeCards - Player's hole cards
 * @param {object} communityCards - Community cards
 * @returns {object} - {flushDraw, straightDraw, outs, equity}
 */
export function calculateDraws(holeCards, communityCards = {}) {
  const parsed = parseCards(holeCards)
  const community = [
    ...parseCards(communityCards.flop || []),
    ...parseCards(communityCards.turn || []),
    ...parseCards(communityCards.river || [])
  ]
  const allCards = [...parsed, ...community]

  if (community.length === 0) {
    return { flushDraw: 0, straightDraw: 0, outs: 0, equity: 0 }
  }

  // Count suits for flush draw
  const suitCounts = {}
  for (const card of allCards) {
    suitCounts[card.suit] = (suitCounts[card.suit] || 0) + 1
  }

  let flushDraw = 0
  for (const count of Object.values(suitCounts)) {
    if (count === 4) flushDraw = 9 // 9 outs for flush draw
  }

  // Check for straight draw
  const ranks = [...new Set(allCards.map(c => c.rank))]
  const values = ranks.map(r => getRankValue(r)).sort((a, b) => a - b)

  let straightDraw = 0

  // Check for open-ended straight draw (4 consecutive, 8 outs)
  for (let i = 0; i < values.length - 3; i++) {
    if (values[i+3] - values[i] === 3) {
      // Check if open-ended (not blocked at either end)
      const low = values[i]
      const high = values[i+3]
      if (low > 2 && high < 14) {
        straightDraw = 8
        break
      } else {
        straightDraw = Math.max(straightDraw, 4) // Gutshot
      }
    }
  }

  // Check for gutshot (1 card needed, 4 outs)
  if (straightDraw === 0) {
    for (const pattern of STRAIGHT_PATTERNS) {
      const patternValues = pattern.map(r => getRankValue(r))
      const matches = patternValues.filter(v => values.includes(v)).length
      if (matches === 4) {
        straightDraw = 4
        break
      }
    }
  }

  // Calculate total outs and equity
  // Flush draw and straight draw can overlap (combo draw)
  const totalOuts = flushDraw + straightDraw

  // Rule of 2 and 4: multiply outs by 2 (turn) or 4 (turn + river)
  const cardsTocome = community.length === 3 ? 2 : 1
  const equity = Math.min(0.5, (totalOuts * (cardsTocome === 2 ? 4 : 2)) / 100)

  return {
    flushDraw,
    straightDraw,
    outs: totalOuts,
    equity
  }
}

// Export for testing
export const _test = {
  checkStraight,
  getStraightHighCard,
  normalizeStrength,
  STRAIGHT_PATTERNS,
  RANK_VALUES
}
