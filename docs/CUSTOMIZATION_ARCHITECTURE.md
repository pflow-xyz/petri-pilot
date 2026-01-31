# Petri-Pilot Customization Architecture

## Problem Statement

Generated code needs to be regenerated when the model changes, but users also need to customize the UI/behavior without losing changes.

## Current State

- `frontend/src/` - **Regenerated** every time (loses customizations)
- `frontend/custom/` - **Preserved** (SkipIfExists: true)
  - `components.js` - Custom web components
  - `theme.css` - Custom styling

## Proposed Architecture: Extension Points

### 1. Hook-Based Extensions

Generated code defines hooks that custom code can register with:

```javascript
// Generated: src/admin.js
const hooks = {
  renderInstanceActions: [],  // Array of render functions
  onArchive: [],              // Array of callbacks
  onDelete: [],
}

export function registerHook(name, fn) {
  if (hooks[name]) hooks[name].push(fn)
}

// In render code:
function renderActions(instance) {
  const customActions = hooks.renderInstanceActions
    .map(fn => fn(instance))
    .join('')
  return `${defaultActions}${customActions}`
}
```

```javascript
// Custom: custom/extensions.js
import { registerHook } from '/src/admin.js'

registerHook('renderInstanceActions', (instance) => {
  return `<button onclick="exportGame('${instance.id}')">Export</button>`
})
```

### 2. Slot-Based UI Composition

Generated HTML includes named slots:

```javascript
// Generated: renders container with slots
function renderInstanceDetail(instance) {
  return `
    <div class="instance-detail">
      <div data-slot="header">${renderHeader(instance)}</div>
      <div data-slot="state">${renderState(instance)}</div>
      <div data-slot="actions">${renderActions(instance)}</div>
      <div data-slot="custom"></div>  <!-- Empty slot for custom content -->
    </div>
  `
}
```

```javascript
// Custom: fills slots
document.addEventListener('DOMContentLoaded', () => {
  const customSlot = document.querySelector('[data-slot="custom"]')
  if (customSlot) {
    customSlot.innerHTML = renderGameBoard(currentInstance)
  }
})
```

### 3. Configuration in Model Spec

Move more customization into the JSON model:

```json
{
  "name": "tic-tac-toe",
  "admin": {
    "enabled": true,
    "columns": ["id", "status", "x_turn", "o_turn"],
    "actions": ["archive", "delete", "export"]
  },
  "softDelete": {
    "enabled": true,
    "retentionDays": 30
  },
  "customViews": {
    "gameBoard": {
      "component": "game-board",
      "props": ["state"]
    }
  }
}
```

### 4. Override Pattern

Custom files can export overrides that replace generated defaults:

```javascript
// Generated: src/admin.js
import { overrides } from '/custom/admin-overrides.js'

function renderInstanceRow(instance) {
  if (overrides.renderInstanceRow) {
    return overrides.renderInstanceRow(instance)
  }
  return defaultRenderInstanceRow(instance)
}
```

```javascript
// Custom: custom/admin-overrides.js (SkipIfExists)
export const overrides = {
  renderInstanceRow: (instance) => {
    // Completely custom rendering
    return `<tr class="game-row">...</tr>`
  }
}
```

### 5. Layered File Structure

```
frontend/
├── src/                    # REGENERATED - core functionality
│   ├── admin.core.js       # Base admin with extension points
│   ├── main.js             # Imports core + extensions
│   └── ...
├── custom/                 # PRESERVED - user customizations
│   ├── extensions.js       # Hook registrations
│   ├── overrides.js        # Function overrides
│   ├── components.js       # Custom web components
│   └── theme.css           # Custom styling
└── index.html              # Loads in correct order
```

## Implementation Priority

1. **Quick Win: More SkipIfExists files**
   - Add `custom/admin-extensions.js`
   - Add `custom/view-extensions.js`
   - Generated code imports these (empty by default)

2. **Medium: Hook System**
   - Add hook registration to generated admin.js
   - Document available hooks

3. **Long-term: Model-Driven Customization**
   - More features configurable in JSON spec
   - Reduces need for code customization

## Migration Path

For existing generated apps:
1. Copy customizations to `custom/` directory
2. Regenerate (customizations preserved)
3. Wire up extensions in generated code

## File Categories

| File | Location | Behavior | Use For |
|------|----------|----------|---------|
| Core logic | `src/*.js` | Regenerated | Workflow, API calls |
| UI overrides | `custom/overrides.js` | Preserved | Replace default renders |
| Extensions | `custom/extensions.js` | Preserved | Add features |
| Components | `custom/components.js` | Preserved | Custom elements |
| Styling | `custom/theme.css` | Preserved | Visual customization |
