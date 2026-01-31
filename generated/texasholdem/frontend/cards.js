// Card rendering utilities for Texas Hold'em
// Provides functions to render playing cards with suits and ranks

const SUITS = {
  hearts: '♥',
  diamonds: '♦',
  clubs: '♣',
  spades: '♠'
}

const SUIT_COLORS = {
  hearts: 'red',
  diamonds: 'red',
  clubs: 'black',
  spades: 'black'
}

const RANKS = ['2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K', 'A']

/**
 * Parse a card string like "Ah" (Ace of hearts) or "10s" (10 of spades)
 * @param {string} cardStr - Card string (e.g., "Ah", "10s", "Kd", "2c")
 * @returns {Object} - {rank, suit, suitSymbol, color}
 */
export function parseCard(cardStr) {
  if (!cardStr || cardStr.length < 2) {
    return null
  }
  
  const suit = cardStr.slice(-1).toLowerCase()
  const rank = cardStr.slice(0, -1)
  
  const suitMap = {
    'h': 'hearts',
    'd': 'diamonds',
    'c': 'clubs',
    's': 'spades'
  }
  
  const suitName = suitMap[suit]
  if (!suitName) {
    return null
  }
  
  return {
    rank: rank.toUpperCase(),
    suit: suitName,
    suitSymbol: SUITS[suitName],
    color: SUIT_COLORS[suitName]
  }
}

/**
 * Render a single card as HTML
 * @param {string} cardStr - Card string (e.g., "Ah", "10s")
 * @param {boolean} faceDown - Whether to show card face down
 * @returns {string} - HTML string for the card
 */
export function renderCard(cardStr, faceDown = false) {
  if (faceDown) {
    return `
      <div class="card card-back">
        <div class="card-pattern"></div>
      </div>
    `
  }
  
  const card = parseCard(cardStr)
  if (!card) {
    return '<div class="card card-empty"></div>'
  }
  
  return `
    <div class="card card-face card-${card.color}">
      <div class="card-rank">${card.rank}</div>
      <div class="card-suit">${card.suitSymbol}</div>
    </div>
  `
}

/**
 * Render multiple cards
 * @param {Array<string>} cards - Array of card strings
 * @param {boolean} faceDown - Whether to show cards face down
 * @returns {string} - HTML string for all cards
 */
export function renderCards(cards, faceDown = false) {
  if (!cards || cards.length === 0) {
    return ''
  }
  
  return cards.map(card => renderCard(card, faceDown)).join('')
}

/**
 * Render community cards (flop, turn, river)
 * @param {Object} communityCards - {flop: [], turn: [], river: []}
 * @returns {string} - HTML string for community cards
 */
export function renderCommunityCards(communityCards) {
  const cards = []
  
  if (communityCards.flop && communityCards.flop.length > 0) {
    cards.push(...communityCards.flop)
  }
  
  if (communityCards.turn && communityCards.turn.length > 0) {
    cards.push(...communityCards.turn)
  }
  
  if (communityCards.river && communityCards.river.length > 0) {
    cards.push(...communityCards.river)
  }
  
  // Show empty placeholders if no cards yet
  while (cards.length < 5) {
    cards.push(null)
  }
  
  return cards.map(card => {
    if (card) {
      return renderCard(card)
    } else {
      return '<div class="card card-placeholder"></div>'
    }
  }).join('')
}

/**
 * Get card value for comparison (for hand strength)
 * @param {string} cardStr - Card string
 * @returns {number} - Numeric value (2-14)
 */
export function getCardValue(cardStr) {
  const card = parseCard(cardStr)
  if (!card) return 0
  
  const rankValues = {
    '2': 2, '3': 3, '4': 4, '5': 5, '6': 6, '7': 7, '8': 8, '9': 9,
    '10': 10, 'J': 11, 'Q': 12, 'K': 13, 'A': 14
  }
  
  return rankValues[card.rank] || 0
}
