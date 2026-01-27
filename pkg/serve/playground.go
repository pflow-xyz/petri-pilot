// Package serve provides unified GraphQL support for Petri-pilot services.
package serve

import (
	"net/http"
)

// PlaygroundHandler returns an HTTP handler that serves the GraphQL Playground UI.
func PlaygroundHandler(endpoint string) http.HandlerFunc {
	html := `<!DOCTYPE html>
<html>
<head>
  <title>Petri-Pilot GraphQL Playground</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/css/index.css" />
  <link rel="shortcut icon" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/favicon.png" />
  <script src="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/js/middleware.js"></script>
  <style>
    #ops-tab {
      display: flex; align-items: center;
      padding: 10px 14px; cursor: pointer; font-size: 12px;
      color: rgba(255,255,255,0.6); background: transparent;
      border: none; border-right: 1px solid rgba(255,255,255,0.06);
      font-family: 'Source Code Pro', 'Consolas', 'Inconsolata', 'Droid Sans Mono', 'Monaco', monospace;
      gap: 6px; white-space: nowrap; flex-shrink: 0;
      margin-right: 6px;
    }
    #ops-tab:hover { color: #fff; background: rgba(255,255,255,0.06); }
    #ops-tab.active { color: #fff; background: rgb(23, 43, 58); }
    #ops-tab .ops-tab-icon { font-size: 12px; }
    #ops-sidebar {
      width: 0; overflow: hidden; flex-shrink: 0;
      background: #0b1924; color: #ccc;
      border-right: none;
      font-family: 'Source Code Pro', monospace; font-size: 13px;
      transition: width 0.2s ease;
    }
    #ops-sidebar.open { width: 320px; overflow-y: auto; border-right: 1px solid rgba(255,255,255,0.06); }
    #ops-sidebar .ops-header {
      padding: 10px 16px; font-size: 13px; font-weight: bold; color: rgba(255,255,255,0.5);
      border-bottom: 1px solid rgba(255,255,255,0.06);
      text-transform: uppercase; letter-spacing: 0.5px;
    }
    #ops-sidebar .ops-group { margin-bottom: 4px; }
    #ops-sidebar .ops-group-title {
      padding: 8px 16px; font-weight: bold; color: rgba(255,255,255,0.4); font-size: 11px;
      text-transform: uppercase; letter-spacing: 0.5px; cursor: pointer;
      display: flex; align-items: center; gap: 6px;
    }
    #ops-sidebar .ops-group-title:hover { color: #fff; }
    #ops-sidebar .ops-group-title .arrow { font-size: 9px; transition: transform 0.15s; }
    #ops-sidebar .ops-group-title.collapsed .arrow { transform: rotate(-90deg); }
    #ops-sidebar .ops-section-title {
      padding: 4px 16px 4px 24px; font-size: 10px; color: #6a9955;
      text-transform: uppercase; letter-spacing: 0.8px;
    }
    #ops-sidebar .ops-section-title.mutation-section { color: #d4a054; }
    #ops-sidebar .ops-item {
      padding: 5px 16px 5px 32px; cursor: pointer; display: flex;
      align-items: center; gap: 8px; white-space: nowrap;
      overflow: hidden; text-overflow: ellipsis;
    }
    #ops-sidebar .ops-item:hover { background: rgba(255,255,255,0.06); color: #fff; }
    #ops-sidebar .ops-item .play-icon { color: #6a9955; font-size: 10px; flex-shrink: 0; }
    #ops-sidebar .ops-item.mutation-item .play-icon { color: #d4a054; }
    #ops-sidebar .ops-item .op-name { overflow: hidden; text-overflow: ellipsis; }
    #ops-filter-wrap {
      padding: 8px 12px; border-bottom: 1px solid rgba(255,255,255,0.06);
      position: sticky; top: 0; background: #0b1924; z-index: 1;
    }
    #ops-filter {
      width: 100%; box-sizing: border-box; padding: 6px 10px;
      background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1);
      border-radius: 4px; color: #ccc; font-size: 13px;
      font-family: 'Source Code Pro', monospace; outline: none;
    }
    #ops-filter:focus { border-color: rgba(255,255,255,0.3); }
    #ops-filter::placeholder { color: rgba(255,255,255,0.3); }

    /* Models tab — matches Docs/Schema vertical rotated tabs */
    #models-tab {
      display: block; padding: 8px; cursor: pointer; font-size: 12px;
      color: rgba(255,255,255,0.6); background: transparent;
      font-family: 'Open Sans', sans-serif; white-space: nowrap;
      text-transform: uppercase; letter-spacing: 0.45px;
      transform: rotate(-90deg);
      transform-origin: center center;
    }
    #models-tab:hover { color: #fff; }
    #models-tab.active { color: #fff; }

    #models-panel {
      position: absolute; right: 0; top: 0; bottom: 0; width: 0;
      overflow: hidden; background: #0b1924; color: #ccc; z-index: 10;
      font-family: 'Source Code Pro', monospace; font-size: 13px;
      transition: width 0.2s ease; border-left: none;
    }
    #models-panel.open { width: 520px; overflow-y: auto; border-left: 1px solid rgba(255,255,255,0.06); }

    #models-panel .mp-header {
      padding: 10px 16px; font-size: 13px; font-weight: bold;
      color: rgba(255,255,255,0.5); border-bottom: 1px solid rgba(255,255,255,0.06);
      text-transform: uppercase; letter-spacing: 0.5px;
      display: flex; align-items: center; justify-content: space-between;
    }
    #models-panel .mp-back {
      cursor: pointer; color: rgba(255,255,255,0.5); font-size: 13px;
      background: none; border: none; font-family: inherit; padding: 2px 8px;
    }
    #models-panel .mp-back:hover { color: #fff; }

    #models-panel .mp-card {
      padding: 12px 16px; cursor: pointer;
      border-bottom: 1px solid rgba(255,255,255,0.04);
      display: flex; align-items: center; gap: 10px;
    }
    #models-panel .mp-card:hover { background: rgba(255,255,255,0.06); color: #fff; }
    #models-panel .mp-card .mp-icon { color: #6a9955; font-size: 16px; flex-shrink: 0; }
    #models-panel .mp-card .mp-name { font-weight: 500; }
    #models-panel .mp-card .mp-arrow { margin-left: auto; color: rgba(255,255,255,0.3); }

    #models-panel .mp-detail { padding: 16px; }
    #models-panel .mp-detail h3 {
      margin: 0 0 4px; color: #fff; font-size: 15px;
    }
    #models-panel .mp-detail .mp-meta {
      color: rgba(255,255,255,0.4); font-size: 11px; margin-bottom: 12px;
    }
    #models-panel .mp-detail .mp-desc {
      color: rgba(255,255,255,0.6); font-size: 12px; margin-bottom: 16px; line-height: 1.5;
    }
    #models-panel .mp-svg-wrap {
      background: #0d2137; border-radius: 6px; padding: 12px; margin-bottom: 16px;
      text-align: center; overflow-x: auto;
    }
    #models-panel .mp-svg-wrap svg {
      max-width: 100%; height: auto;
    }
    #models-panel .mp-section-title {
      font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px;
      color: rgba(255,255,255,0.4); margin: 16px 0 8px; font-weight: bold;
    }
    #models-panel .mp-collapsible-title {
      font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px;
      color: rgba(255,255,255,0.4); margin: 16px 0 0; font-weight: bold;
      cursor: pointer; display: flex; align-items: center; gap: 6px;
      padding: 6px 0;
    }
    #models-panel .mp-collapsible-title:hover { color: rgba(255,255,255,0.7); }
    #models-panel .mp-collapsible-title .mp-arrow {
      font-size: 9px; transition: transform 0.15s; display: inline-block;
    }
    #models-panel .mp-collapsible-title.collapsed .mp-arrow { transform: rotate(-90deg); }
    #models-panel .mp-collapsible-body { overflow: hidden; }
    #models-panel .mp-collapsible-body.collapsed { display: none; }
    #models-panel table.mp-table {
      width: 100%; border-collapse: collapse; font-size: 12px; margin-bottom: 12px;
    }
    #models-panel table.mp-table th {
      text-align: left; padding: 4px 8px; color: rgba(255,255,255,0.4);
      border-bottom: 1px solid rgba(255,255,255,0.1); font-weight: normal;
      text-transform: uppercase; font-size: 10px; letter-spacing: 0.5px;
    }
    #models-panel table.mp-table td {
      padding: 4px 8px; border-bottom: 1px solid rgba(255,255,255,0.04);
      color: rgba(255,255,255,0.7);
    }
    #models-panel table.mp-table tr:hover td { color: #fff; }
    #models-panel .mp-event {
      margin-bottom: 10px; padding: 8px 12px;
      background: rgba(255,255,255,0.03); border-radius: 4px;
      border-left: 2px solid #6a9955;
    }
    #models-panel .mp-event-name {
      font-weight: 500; color: rgba(255,255,255,0.8); margin-bottom: 2px;
    }
    #models-panel .mp-event-desc {
      font-size: 11px; color: rgba(255,255,255,0.4); margin-bottom: 6px;
    }
    #models-panel .mp-event .mp-table { margin-bottom: 0; }
    #models-panel code {
      font-size: 11px; color: #d4a054; background: rgba(255,255,255,0.05);
      padding: 1px 4px; border-radius: 2px;
    }
    #models-panel .mp-pflow-btn {
      display: inline-flex; align-items: center; gap: 6px;
      padding: 6px 14px; margin-bottom: 12px;
      background: rgba(255,255,255,0.08); border: 1px solid rgba(255,255,255,0.15);
      border-radius: 6px; cursor: pointer; text-decoration: none;
      color: rgba(255,255,255,0.7); font-size: 12px;
      font-family: 'Source Code Pro', monospace;
      transition: background 0.15s, border-color 0.15s;
    }
    #models-panel .mp-pflow-btn:hover {
      background: rgba(255,255,255,0.14); border-color: rgba(255,255,255,0.3);
      color: #fff;
    }
    #models-panel .mp-pflow-btn svg {
      height: 16px; width: auto; fill: currentColor;
      vertical-align: -2px;
    }
  </style>
</head>
<body>
  <div id="root"></div>
  <div id="ops-sidebar"><div class="ops-header">Operations Explorer</div><div id="ops-filter-wrap"><input id="ops-filter" type="text" placeholder="Filter operations..." /></div><div id="ops-content"></div></div>
  <div id="models-panel"><div class="mp-header"><span>Models</span></div><div id="models-content"></div></div>
  <script>
    window.addEventListener('load', function() {
      GraphQLPlayground.init(document.getElementById('root'), {
        endpoint: '` + endpoint + `',
        settings: {
          'editor.theme': 'dark',
          'editor.fontFamily': "'Source Code Pro', 'Consolas', 'Inconsolata', 'Droid Sans Mono', 'Monaco', monospace",
          'editor.fontSize': 14,
          'request.credentials': 'same-origin'
        }
      })
    })
  </script>
  <script>
  (function() {
    var sidebar = document.getElementById('ops-sidebar');

    // Inject toggle into the tab bar and move sidebar into the editor area
    function injectOpsTab() {
      var tabRow = document.querySelector('.playground .sc-bXGyLb');
      if (!tabRow) { setTimeout(injectOpsTab, 200); return; }
      // Create the tab button
      var btn = document.createElement('div');
      btn.id = 'ops-tab';
      btn.innerHTML = '<span class="ops-tab-icon">&#9776;</span> Ops';
      btn.title = 'Operations Explorer';
      tabRow.insertBefore(btn, tabRow.firstChild);
      btn.addEventListener('click', function() {
        var isOpen = sidebar.classList.toggle('open');
        btn.classList.toggle('active', isOpen);
      });
      // Insert sidebar into the editor area as a flex sibling to push content
      var editorArea = document.querySelector('.sc-VJcYb');
      if (editorArea) {
        editorArea.style.display = 'flex';
        editorArea.style.flexDirection = 'row';
        // Make existing children fill remaining space
        for (var c = 0; c < editorArea.children.length; c++) {
          editorArea.children[c].style.flex = '1 1 auto';
          editorArea.children[c].style.minWidth = '0';
        }
        editorArea.insertBefore(sidebar, editorArea.firstChild);
      }
    }
    injectOpsTab();

    function defaultVal(type) {
      type = type.replace(/!/g, '');
      if (type === 'Int' || type === 'Float') return '1';
      if (type === 'Boolean') return 'true';
      if (type === 'ID') return '"test-id"';
      if (type === 'String') return '""';
      if (type === 'Time') return '"2025-01-01T00:00:00Z"';
      return 'null';
    }

    function isScalar(t) {
      t = t.replace(/[!\[\]]/g, '');
      return ['Int','Float','String','Boolean','ID','Time'].indexOf(t) >= 0;
    }

    function unwrapType(t) {
      return t.replace(/[!\[\]]/g, '');
    }

    function parseSDL(sdl) {
      var types = {};
      var inputTypes = {};
      // Parse type and input definitions
      var typeRe = /(type|input)\s+(\w+)\s*\{([^}]*)\}/g;
      var m;
      while ((m = typeRe.exec(sdl)) !== null) {
        var kind = m[1];
        var tname = m[2];
        var body = m[3];
        var fields = [];
        var fRe = /(\w+)(?:\(([^)]*)\))?\s*:\s*([^\n,]+)/g;
        var fm;
        while ((fm = fRe.exec(body)) !== null) {
          var args = [];
          if (fm[2]) {
            var aRe = /(\w+)\s*:\s*([^\s,]+)/g;
            var am;
            while ((am = aRe.exec(fm[2])) !== null) {
              args.push({ name: am[1], type: am[2].trim() });
            }
          }
          fields.push({ name: fm[1], args: args, type: fm[3].trim() });
        }
        types[tname] = fields;
        if (kind === 'input') inputTypes[tname] = fields;
      }
      types._inputs = inputTypes;
      return types;
    }

    function buildFieldSelection(typeName, types, depth) {
      depth = depth || 0;
      var t = types[unwrapType(typeName)];
      if (!t) return '    id';
      var lines = [];
      for (var i = 0; i < t.length; i++) {
        var f = t[i];
        if (f.args.length > 0) continue; // skip fields with args in nested
        var indent = '    ';
        for (var d = 0; d < depth; d++) indent += '  ';
        if (isScalar(f.type)) {
          lines.push(indent + f.name);
        } else if (depth < 1) {
          var inner = buildFieldSelection(f.type, types, depth + 1);
          if (inner) lines.push(indent + f.name + ' {\n' + inner + '\n' + indent + '}');
        }
      }
      return lines.join('\n');
    }

    function buildInputObject(typeName, types) {
      var inputs = types._inputs || {};
      var fields = inputs[unwrapType(typeName)];
      if (!fields) return defaultVal(typeName);
      var parts = [];
      for (var i = 0; i < fields.length; i++) {
        var f = fields[i];
        var val = defaultVal(f.type);
        // Check if the field's type is itself an input type
        if (!isScalar(f.type) && inputs[unwrapType(f.type)]) {
          val = buildInputObject(f.type, types);
        }
        parts.push(f.name + ': ' + val);
      }
      return '{ ' + parts.join(', ') + ' }';
    }

    function generateQuery(op, kind, types) {
      var inputs = types._inputs || {};
      var argsStr = '';
      if (op.args.length > 0) {
        var parts = [];
        for (var i = 0; i < op.args.length; i++) {
          var arg = op.args[i];
          var rawType = unwrapType(arg.type);
          if (inputs[rawType]) {
            parts.push(arg.name + ': ' + buildInputObject(arg.type, types));
          } else {
            parts.push(arg.name + ': ' + defaultVal(arg.type));
          }
        }
        argsStr = '(' + parts.join(', ') + ')';
      }
      var fields = buildFieldSelection(op.type, types, 0);
      var body = '  ' + op.name + argsStr;
      if (!isScalar(op.type)) {
        body += ' {\n' + fields + '\n  }';
      }
      if (kind === 'mutation') {
        return 'mutation {\n' + body + '\n}';
      }
      return '{\n' + body + '\n}';
    }

    function closeSidebar() {
      sidebar.classList.remove('open');
      var tab = document.getElementById('ops-tab');
      if (tab) tab.classList.remove('active');
    }

    function setEditorValue(text) {
      var cm = document.querySelector('.CodeMirror');
      if (cm && cm.CodeMirror) {
        cm.CodeMirror.setValue(text);
      }
    }

    function groupByService(ops, knownServices) {
      var groups = {};
      for (var i = 0; i < ops.length; i++) {
        var op = ops[i];
        var service = null;
        // Try underscore prefix first (mutations: blogpost_create)
        var uparts = op.name.match(/^([a-z][a-z0-9]*)_/);
        if (uparts) {
          service = uparts[1];
        }
        // Try camelCase prefix (queries: blogpostList)
        if (!service) {
          var cparts = op.name.match(/^([a-z]+[a-z0-9]*?)([A-Z].*)/);
          if (cparts) {
            service = cparts[1];
          }
        }
        // Match bare name against known services (e.g. query "blogpost" -> "blogpost")
        if (!service && knownServices) {
          for (var s = 0; s < knownServices.length; s++) {
            if (op.name === knownServices[s]) { service = op.name; break; }
          }
        }
        if (!service) service = 'other';
        if (!groups[service]) groups[service] = [];
        groups[service].push(op);
      }
      return groups;
    }

    function render(queries, mutations, types) {
      var container = document.getElementById('ops-content');
      container.innerHTML = '';

      var allOps = [
        { label: 'Queries', items: queries, kind: 'query' },
        { label: 'Mutations', items: mutations, kind: 'mutation' }
      ];

      // Detect known service prefixes from mutations (they use underscore: blogpost_create)
      var knownSvcSet = {};
      for (var mi = 0; mi < mutations.length; mi++) {
        var up = mutations[mi].name.match(/^([a-z][a-z0-9]*)_/);
        if (up) knownSvcSet[up[1]] = true;
      }
      var knownServices = Object.keys(knownSvcSet);

      // Collect all services across queries and mutations
      var serviceSet = {};
      var qGroups = groupByService(queries, knownServices);
      var mGroups = groupByService(mutations, knownServices);
      for (var k in qGroups) serviceSet[k] = true;
      for (var k in mGroups) serviceSet[k] = true;
      var services = Object.keys(serviceSet).sort();

      for (var si = 0; si < services.length; si++) {
        var svc = services[si];
        var group = document.createElement('div');
        group.className = 'ops-group';

        var title = document.createElement('div');
        title.className = 'ops-group-title';
        title.innerHTML = '<span class="arrow">&#9660;</span> ' + svc;
        title.addEventListener('click', (function(g) {
          return function() {
            this.classList.toggle('collapsed');
            var items = g.querySelectorAll('.ops-section');
            for (var x = 0; x < items.length; x++) {
              items[x].style.display = this.classList.contains('collapsed') ? 'none' : '';
            }
          };
        })(group));
        group.appendChild(title);

        var svcQueries = qGroups[svc] || [];
        var svcMutations = mGroups[svc] || [];

        if (svcQueries.length > 0) {
          var sec = document.createElement('div');
          sec.className = 'ops-section';
          var secTitle = document.createElement('div');
          secTitle.className = 'ops-section-title';
          secTitle.textContent = 'Queries';
          sec.appendChild(secTitle);
          for (var qi = 0; qi < svcQueries.length; qi++) {
            var item = document.createElement('div');
            item.className = 'ops-item';
            item.innerHTML = '<span class="play-icon">&#9654;</span><span class="op-name">' + svcQueries[qi].name + '</span>';
            item.addEventListener('click', (function(op, kind) {
              return function() {
                setEditorValue(generateQuery(op, kind, types));
                closeSidebar();
              };
            })(svcQueries[qi], 'query'));
            sec.appendChild(item);
          }
          group.appendChild(sec);
        }

        if (svcMutations.length > 0) {
          var msec = document.createElement('div');
          msec.className = 'ops-section';
          var msecTitle = document.createElement('div');
          msecTitle.className = 'ops-section-title mutation-section';
          msecTitle.textContent = 'Mutations';
          msec.appendChild(msecTitle);
          for (var mi = 0; mi < svcMutations.length; mi++) {
            var mitem = document.createElement('div');
            mitem.className = 'ops-item mutation-item';
            mitem.innerHTML = '<span class="play-icon">&#9654;</span><span class="op-name">' + svcMutations[mi].name + '</span>';
            mitem.addEventListener('click', (function(op, kind) {
              return function() {
                setEditorValue(generateQuery(op, kind, types));
                closeSidebar();
              };
            })(svcMutations[mi], 'mutation'));
            msec.appendChild(mitem);
          }
          group.appendChild(msec);
        }

        container.appendChild(group);
      }
    }

    // Filter logic
    var filterInput = document.getElementById('ops-filter');
    filterInput.addEventListener('input', function() {
      var q = this.value.toLowerCase();
      var items = document.querySelectorAll('#ops-content .ops-item');
      var groups = document.querySelectorAll('#ops-content .ops-group');
      // Show/hide individual items
      for (var i = 0; i < items.length; i++) {
        var name = items[i].querySelector('.op-name');
        var match = !q || (name && name.textContent.toLowerCase().indexOf(q) >= 0);
        items[i].style.display = match ? '' : 'none';
      }
      // Show/hide sections based on whether they have visible items
      var sections = document.querySelectorAll('#ops-content .ops-section');
      for (var s = 0; s < sections.length; s++) {
        var visibleItems = sections[s].querySelectorAll('.ops-item:not([style*="display: none"])');
        sections[s].style.display = visibleItems.length > 0 ? '' : 'none';
      }
      // Show/hide groups based on whether they have visible sections
      for (var g = 0; g < groups.length; g++) {
        var visibleSections = groups[g].querySelectorAll('.ops-section:not([style*="display: none"])');
        groups[g].style.display = visibleSections.length > 0 ? '' : 'none';
      }
    });

    // Fetch schema and build the sidebar
    fetch('/schema')
      .then(function(r) { return r.text(); })
      .then(function(sdl) {
        var types = parseSDL(sdl);
        var queries = types['Query'] || [];
        var mutations = types['Mutation'] || [];
        delete types['Query'];
        delete types['Mutation'];
        render(queries, mutations, types);
      })
      .catch(function(err) {
        console.error('Failed to load schema for ops explorer:', err);
      });
  })();
  </script>
  <script>
  (function() {
    var modelsPanel = document.getElementById('models-panel');
    var modelsContent = document.getElementById('models-content');
    var modelsHeader = modelsPanel.querySelector('.mp-header');
    var modelsCache = {};

    function injectModelsTab() {
      // Find the Schema tab (last tab in the right-side vertical bar)
      var schemaTab = null;
      var allDivs = document.querySelectorAll('.playground .sc-TOsTZ');
      for (var i = 0; i < allDivs.length; i++) {
        if (allDivs[i].textContent.trim() === 'Schema') {
          schemaTab = allDivs[i];
          break;
        }
      }
      if (!schemaTab) { setTimeout(injectModelsTab, 300); return; }

      // Clone the Schema tab to inherit all its styled-component classes
      var btn = schemaTab.cloneNode(true);
      btn.id = 'models-tab';
      btn.textContent = 'Models';
      btn.title = 'Petri Net Models';
      // Match the transform-origin used by Docs/Schema tabs so rotation pivots correctly
      var schemaStyle = window.getComputedStyle(schemaTab);
      btn.style.transformOrigin = schemaStyle.transformOrigin;
      btn.style.whiteSpace = schemaStyle.whiteSpace;
      schemaTab.parentElement.appendChild(btn);

      // Ensure the right tab bar container sits above the models panel
      var rightTabContainer = schemaTab.parentElement.parentElement;
      if (rightTabContainer) {
        rightTabContainer.style.zIndex = '30';
      }

      btn.addEventListener('click', function() {
        var isOpen = modelsPanel.classList.toggle('open');
        btn.classList.toggle('active', isOpen);
        if (isOpen) loadModelsList();
      });

      // Position the panel inside the playground editor area
      var editorArea = document.querySelector('.sc-VJcYb');
      if (editorArea) {
        editorArea.style.position = 'relative';
        editorArea.appendChild(modelsPanel);
      }
    }
    injectModelsTab();

    function loadModelsList() {
      fetch('/models')
        .then(function(r) { return r.json(); })
        .then(function(names) {
          renderList(names);
        })
        .catch(function(err) {
          modelsContent.innerHTML = '<div style="padding:16px;color:#e57373;">Failed to load models</div>';
        });
    }

    function renderList(names) {
      modelsHeader.innerHTML = '<span>Models</span>';
      modelsContent.innerHTML = '';
      for (var i = 0; i < names.length; i++) {
        var card = document.createElement('div');
        card.className = 'mp-card';
        card.innerHTML = '<span class="mp-icon">&#9673;</span><span class="mp-name">' + escHtml(names[i]) + '</span><span class="mp-arrow">&#9654;</span>';
        card.addEventListener('click', (function(name) {
          return function() { loadModelDetail(name); };
        })(names[i]));
        modelsContent.appendChild(card);
      }
    }

    function loadModelDetail(name) {
      if (modelsCache[name]) {
        renderDetail(name, modelsCache[name]);
        return;
      }
      modelsContent.innerHTML = '<div style="padding:16px;color:rgba(255,255,255,0.5);">Loading...</div>';
      fetch('/' + name + '/api/schema')
        .then(function(r) { return r.json(); })
        .then(function(model) {
          modelsCache[name] = model;
          renderDetail(name, model);
        })
        .catch(function(err) {
          modelsContent.innerHTML = '<div style="padding:16px;color:#e57373;">Failed to load model: ' + escHtml(name) + '</div>';
        });
    }

    var collapsibleId = 0;
    function collapsibleSection(title, count, contentHtml, collapsed) {
      var id = 'mp-collapse-' + (collapsibleId++);
      var cls = collapsed ? ' collapsed' : '';
      var html = '<div class="mp-collapsible-title' + cls + '" onclick="var b=document.getElementById(\'' + id + '\');this.classList.toggle(\'collapsed\');b.classList.toggle(\'collapsed\')">';
      html += '<span class="mp-arrow">&#9660;</span> ' + escHtml(title) + ' (' + count + ')</div>';
      html += '<div id="' + id + '" class="mp-collapsible-body' + cls + '">' + contentHtml + '</div>';
      return html;
    }

    function renderDetail(name, model) {
      collapsibleId = 0;
      modelsHeader.innerHTML = '<button class="mp-back">&#9664; Back</button><span>' + escHtml(name) + '</span><span></span>';
      modelsHeader.querySelector('.mp-back').addEventListener('click', function(e) {
        e.stopPropagation();
        loadModelsList();
      });

      var html = '<div class="mp-detail">';
      html += '<h3>' + escHtml(model.name || name) + '</h3>';
      var meta = [];
      if (model.version) meta.push('v' + model.version);
      if (model.type) meta.push(model.type);
      if (meta.length) html += '<div class="mp-meta">' + meta.map(escHtml).join(' \u00b7 ') + '</div>';
      if (model.description) html += '<div class="mp-desc">' + escHtml(model.description) + '</div>';

      // Open in pflow button
      html += '<a class="mp-pflow-btn" href="/pflow?model=' + encodeURIComponent(name) + '" target="_blank">' +
        'Open in <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 490 115"><g transform="translate(5,5)"><path d="M100.88 28.02H78.46v5.61h-5.6v5.6h-5.6v-5.6h5.6v-5.61h5.6V5.6h-5.6V0H61.65v5.6h-5.6v28.02h-5.6V5.6h-5.6V0H33.64v5.6h-5.6v22.42h5.6v5.61h5.6v5.6h-5.6v-5.6h-5.6v-5.61H5.6v5.61H0v11.21h5.6v5.6h28.02v5.6H5.6v5.61H0v11.21h5.6v5.6h22.42v-5.6h5.6v-5.61h5.6v5.61h-5.6v5.6h-5.6v22.42h5.6v5.6h11.21v-5.6h5.6V72.86h5.6v28.02h5.6v5.6h11.21v-5.6h5.6V78.46h-5.6v-5.6h-5.6v-5.61h5.6v5.61h5.6v5.6h22.42v-5.6h5.6V61.65h-5.6v-5.61H72.84v-5.6h28.02v-5.6h5.6V33.63h-5.6v-5.61zM67.25 56.04v5.61h-5.6v5.6H44.84v-5.6h-5.6V44.84h5.6v-5.6h16.81v5.6h5.6v11.21zm89.89-28.02h-11.21v11.21h11.21zm33.63 11.21h11.21V28.02h-33.63v11.21z"/><path d="M179.56 72.86h-11.21V39.23h-11.21v56.05h-11.21v11.21h33.63V95.28h-11.21V84.07h33.63V72.86zm22.42-22.42v22.42h11.21V39.23h-11.21zm33.63-22.42H224.4v11.21h11.21v33.63H224.4v11.21h33.63V72.86h-11.21V39.23h11.21V28.02h-11.21V16.81h-11.21z"/><path d="M246.82 5.6v11.21h22.42V5.6zm56.05 56.05V5.6h-22.42v11.21h11.21v56.05h-11.21v11.21h33.63V72.86h-11.21zm33.63-11.21V39.23h-11.21v33.63h11.21zm22.42 0h-11.21v11.21h11.21zm0-11.21h11.21V28.02H336.5v11.21zm-11.21 33.63H336.5v11.21h33.63V72.86zm22.42-22.42v22.42h11.21V39.23h-11.21zm44.84-11.21V28.02h-22.42v11.21h11.21v22.42h11.21zm11.21 22.42h-11.21v11.21h11.21zm11.21 11.21h-11.21v11.21h11.21zm11.21-22.42V28.02h-11.21v44.84h11.21zm11.21 22.42H448.6v11.21h11.21zm11.21-11.21h-11.21v11.21h11.21zm11.21-33.63h-11.21v33.63h11.21V39.23h11.21V28.02z"/></g></svg></a>';

      // SVG
      html += '<div class="mp-svg-wrap" id="mp-svg-container"></div>';

      // Events (right below graph)
      var events = model.events || [];
      if (events.length > 0) {
        html += '<div class="mp-section-title">Events (' + events.length + ')</div>';
        for (var ei = 0; ei < events.length; ei++) {
          var ev = events[ei];
          html += '<div class="mp-event">';
          html += '<div class="mp-event-name">' + escHtml(ev.id || ev.name || '') + '</div>';
          if (ev.description) html += '<div class="mp-event-desc">' + escHtml(ev.description) + '</div>';
          var fields = ev.fields || [];
          if (fields.length > 0) {
            html += '<table class="mp-table"><tr><th>Field</th><th>Type</th></tr>';
            for (var fi = 0; fi < fields.length; fi++) {
              html += '<tr><td>' + escHtml(fields[fi].name || '') + '</td><td><code>' + escHtml(fields[fi].type || '') + '</code></td></tr>';
            }
            html += '</table>';
          }
          html += '</div>';
        }
      }

      // Roles table
      var roles = model.roles || [];
      if (roles.length > 0) {
        html += '<div class="mp-section-title">Roles (' + roles.length + ')</div>';
        html += '<table class="mp-table"><tr><th>ID</th><th>Description</th></tr>';
        for (var ri = 0; ri < roles.length; ri++) {
          var role = roles[ri];
          html += '<tr><td>' + escHtml(role.id || role.name || '') + '</td><td>' + escHtml(role.description || '-') + '</td></tr>';
        }
        html += '</table>';
      }

      // Access rules table
      var access = model.access || [];
      if (access.length > 0) {
        html += '<div class="mp-section-title">Access Rules (' + access.length + ')</div>';
        html += '<table class="mp-table"><tr><th>Transition</th><th>Role</th></tr>';
        for (var ai = 0; ai < access.length; ai++) {
          var rule = access[ai];
          html += '<tr><td>' + escHtml(rule.transition || '') + '</td><td>' + escHtml(rule.role || '') + '</td></tr>';
        }
        html += '</table>';
      }

      // Places table (collapsed by default)
      var places = model.places || [];
      if (places.length > 0) {
        var ptHtml = '<table class="mp-table"><tr><th>ID</th><th>Initial</th><th>Kind</th></tr>';
        for (var i = 0; i < places.length; i++) {
          var p = places[i];
          ptHtml += '<tr><td>' + escHtml(p.id || '') + '</td><td>' + (p.initial || 0) + '</td><td>' + escHtml(p.kind || '-') + '</td></tr>';
        }
        ptHtml += '</table>';
        html += collapsibleSection('Places', places.length, ptHtml, true);
      }

      // Transitions table (collapsed by default)
      var transitions = model.transitions || [];
      if (transitions.length > 0) {
        var ttHtml = '<table class="mp-table"><tr><th>ID</th><th>Guard</th><th>Event</th></tr>';
        for (var i = 0; i < transitions.length; i++) {
          var t = transitions[i];
          ttHtml += '<tr><td>' + escHtml(t.id || '') + '</td><td><code>' + escHtml(t.guard || '-') + '</code></td><td>' + escHtml(t.event || '-') + '</td></tr>';
        }
        ttHtml += '</table>';
        html += collapsibleSection('Transitions', transitions.length, ttHtml, true);
      }

      // Arcs table (collapsed by default)
      var arcs = model.arcs || [];
      if (arcs.length > 0) {
        var atHtml = '<table class="mp-table"><tr><th>From</th><th>To</th><th>Weight</th></tr>';
        for (var i = 0; i < arcs.length; i++) {
          var a = arcs[i];
          atHtml += '<tr><td>' + escHtml(a.from || '') + '</td><td>' + escHtml(a.to || '') + '</td><td>' + (a.weight || 1) + '</td></tr>';
        }
        atHtml += '</table>';
        html += collapsibleSection('Arcs', arcs.length, atHtml, true);
      }

      html += '</div>';
      modelsContent.innerHTML = html;

      // Render SVG into the container
      var svgContainer = document.getElementById('mp-svg-container');
      if (svgContainer) {
        svgContainer.innerHTML = generateModelSVG(model);
      }
    }

    function generateModelSVG(model) {
      var places = model.places || [];
      var transitions = model.transitions || [];
      var arcs = model.arcs || [];

      // Check for explicit positions (use !== undefined to allow 0 values)
      var hasPositions = false;
      for (var i = 0; i < places.length; i++) {
        if (places[i].x !== undefined || places[i].y !== undefined) { hasPositions = true; break; }
      }
      if (!hasPositions) {
        for (var i = 0; i < transitions.length; i++) {
          if (transitions[i].x !== undefined || transitions[i].y !== undefined) { hasPositions = true; break; }
        }
      }

      var nodePos = {}; // id -> [x, y]
      var nodeType = {}; // id -> 'place' | 'transition'
      for (var i = 0; i < places.length; i++) nodeType[places[i].id] = 'place';
      for (var i = 0; i < transitions.length; i++) nodeType[transitions[i].id] = 'transition';

      if (hasPositions) {
        // Use explicit positions
        for (var i = 0; i < places.length; i++) {
          var p = places[i];
          nodePos[p.id] = [p.x !== undefined ? p.x : 50 + i * 120, p.y !== undefined ? p.y : 50];
        }
        for (var i = 0; i < transitions.length; i++) {
          var t = transitions[i];
          nodePos[t.id] = [t.x !== undefined ? t.x : 50 + i * 120, t.y !== undefined ? t.y : 150];
        }
      } else {
        // Compute layered layout from arcs using topological ordering
        // Build adjacency: place -> transition -> place -> ...
        var outEdges = {};
        var inEdges = {};
        var allIds = [];
        for (var i = 0; i < places.length; i++) { allIds.push(places[i].id); outEdges[places[i].id] = []; inEdges[places[i].id] = []; }
        for (var i = 0; i < transitions.length; i++) { allIds.push(transitions[i].id); outEdges[transitions[i].id] = []; inEdges[transitions[i].id] = []; }
        for (var i = 0; i < arcs.length; i++) {
          var a = arcs[i];
          if (outEdges[a.from]) outEdges[a.from].push(a.to);
          if (inEdges[a.to]) inEdges[a.to].push(a.from);
        }

        // Assign layers using longest-path from sources
        var layer = {};
        var visited = {};
        function assignLayer(id) {
          if (visited[id]) return layer[id] || 0;
          visited[id] = true;
          var maxPrev = -1;
          var ins = inEdges[id] || [];
          for (var j = 0; j < ins.length; j++) {
            var pl = assignLayer(ins[j]);
            if (pl > maxPrev) maxPrev = pl;
          }
          layer[id] = maxPrev + 1;
          return layer[id];
        }
        for (var i = 0; i < allIds.length; i++) assignLayer(allIds[i]);

        // Group nodes by layer
        var layers = {};
        var maxLayer = 0;
        for (var i = 0; i < allIds.length; i++) {
          var l = layer[allIds[i]] || 0;
          if (!layers[l]) layers[l] = [];
          layers[l].push(allIds[i]);
          if (l > maxLayer) maxLayer = l;
        }

        // Position nodes: layers go left-to-right, nodes within layer top-to-bottom
        var colSpacing = 120;
        var rowSpacing = 80;
        var padX = 60, padY = 50;
        for (var l = 0; l <= maxLayer; l++) {
          var nodes = layers[l] || [];
          var totalH = (nodes.length - 1) * rowSpacing;
          var startY = padY + (maxLayer > 0 ? 0 : 0);
          // Center vertically based on max column height
          for (var n = 0; n < nodes.length; n++) {
            nodePos[nodes[n]] = [padX + l * colSpacing, startY + n * rowSpacing];
          }
        }
      }

      // Compute bounds
      var maxX = 0, maxY = 0;
      for (var id in nodePos) {
        if (nodePos[id][0] > maxX) maxX = nodePos[id][0];
        if (nodePos[id][1] > maxY) maxY = nodePos[id][1];
      }
      var width = maxX + 100;
      var height = maxY + 80;

      var svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ' + width + ' ' + height + '">';
      svg += '<defs><marker id="mp-arrow" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">';
      svg += '<polygon points="0 0, 10 3.5, 0 7" fill="#8bb4d6"/></marker></defs>';
      svg += '<style>.mp-place{fill:rgba(30,80,120,0.5);stroke:#4a9eda;stroke-width:2}.mp-trans{fill:#c0c0c0;stroke:#c0c0c0}.mp-label{font-family:system-ui,sans-serif;font-size:10px;text-anchor:middle;fill:#ccc}.mp-arc{stroke:#8bb4d6;stroke-width:1.5;fill:none;marker-end:url(#mp-arrow)}</style>';

      // Draw arcs first (behind nodes)
      for (var i = 0; i < arcs.length; i++) {
        var arc = arcs[i];
        var from = nodePos[arc.from];
        var to = nodePos[arc.to];
        if (!from || !to) continue;
        // Shorten line to avoid overlap with node shapes
        var dx = to[0] - from[0], dy = to[1] - from[1];
        var dist = Math.sqrt(dx * dx + dy * dy);
        if (dist < 1) continue;
        var ux = dx / dist, uy = dy / dist;
        var r1 = nodeType[arc.from] === 'place' ? 22 : 18;
        var r2 = nodeType[arc.to] === 'place' ? 27 : 20;
        var x1 = from[0] + ux * r1, y1 = from[1] + uy * r1;
        var x2 = to[0] - ux * r2, y2 = to[1] - uy * r2;
        svg += '<path d="M' + x1.toFixed(1) + ',' + y1.toFixed(1) + ' L' + x2.toFixed(1) + ',' + y2.toFixed(1) + '" class="mp-arc"/>';
      }

      // Places
      for (var i = 0; i < places.length; i++) {
        var p = places[i];
        var pos = nodePos[p.id];
        if (!pos) continue;
        var x = pos[0], y = pos[1];
        svg += '<circle cx="' + x + '" cy="' + y + '" r="20" class="mp-place"/>';
        svg += '<text x="' + x + '" y="' + (y + 35) + '" class="mp-label">' + escHtml(p.id) + '</text>';
        if (p.initial > 0) {
          svg += '<text x="' + x + '" y="' + (y + 4) + '" class="mp-label" style="font-weight:bold;fill:#fff">' + p.initial + '</text>';
        }
      }

      // Transitions (rounded square, pflow style)
      for (var i = 0; i < transitions.length; i++) {
        var t = transitions[i];
        var pos = nodePos[t.id];
        if (!pos) continue;
        var x = pos[0], y = pos[1];
        svg += '<rect x="' + (x - 12) + '" y="' + (y - 12) + '" width="24" height="24" rx="4" class="mp-trans"/>';
        svg += '<text x="' + x + '" y="' + (y + 30) + '" class="mp-label">' + escHtml(t.id) + '</text>';
      }

      svg += '</svg>';
      return svg;
    }

    function escHtml(s) {
      if (!s) return '';
      return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
    }
  })();
  </script>
</body>
</html>`

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
	}
}

// PflowHandler returns an HTTP handler that serves a read-only Petri net viewer
// using petri-view.js from pflow.xyz. The model name is passed via ?model= query param.
func PflowHandler() http.HandlerFunc {
	pflowHTML := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no" name="viewport"/>
  <title>Petri Net Viewer</title>
  <link href="https://cdn.jsdelivr.net/gh/pflow-xyz/pflow-xyz@latest/public/petri-view.css" rel="stylesheet"/>
  <script src="https://cdn.jsdelivr.net/gh/pflow-xyz/pflow-xyz@latest/public/petri-view.js" type="module"></script>
  <style>
    /* Hide login/share UI — keep all other controls */
    .pv-top-right-btn { display: none !important; }
    .pv-scale-meter, .pv-scale-label, .pv-scale-legend, .pv-scale-reset { z-index: 1; }
    .pv-mode-menu { z-index: 1; }
    .pv-label { color: #222; }
    #pflow-loading {
      position: fixed; top: 0; left: 0; right: 0; bottom: 0;
      display: flex; align-items: center; justify-content: center;
      font-family: system-ui, sans-serif; font-size: 16px; color: #999;
      background: #fafafa; z-index: 950;
    }
    #pflow-error {
      position: fixed; top: 0; left: 0; right: 0; bottom: 0;
      display: none; align-items: center; justify-content: center;
      font-family: system-ui, sans-serif; color: #c0392b;
      background: #fafafa; flex-direction: column; gap: 12px;
    }
    #pflow-error h2 { margin: 0; font-size: 20px; }
    #pflow-error p { margin: 0; font-size: 14px; color: #666; }
    #pflow-error a { color: #2a6fb8; }
    .pflow-title {
      position: fixed; top: 16px; left: 60px; z-index: 1000;
      pointer-events: auto; text-decoration: none;
    }
    .pflow-title svg {
      width: 120px; height: auto; display: block; fill: #ccc;
    }
    .pflow-home {
      position: fixed; top: 20px; left: 200px; z-index: 1000;
      pointer-events: auto; text-decoration: none;
      color: #999; font-family: system-ui, sans-serif; font-size: 13px;
      transition: color 0.15s;
    }
    .pflow-home:hover { color: #fff; }
  </style>
</head>
<body>
  <a class="pflow-title" href="/pflow">
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 490 115"><g transform="translate(5,5)"><path d="M100.88 28.02H78.46v5.61h-5.6v5.6h-5.6v-5.6h5.6v-5.61h5.6V5.6h-5.6V0H61.65v5.6h-5.6v28.02h-5.6V5.6h-5.6V0H33.64v5.6h-5.6v22.42h5.6v5.61h5.6v5.6h-5.6v-5.6h-5.6v-5.61H5.6v5.61H0v11.21h5.6v5.6h28.02v5.6H5.6v5.61H0v11.21h5.6v5.6h22.42v-5.6h5.6v-5.61h5.6v5.61h-5.6v5.6h-5.6v22.42h5.6v5.6h11.21v-5.6h5.6V72.86h5.6v28.02h5.6v5.6h11.21v-5.6h5.6V78.46h-5.6v-5.6h-5.6v-5.61h5.6v5.61h5.6v5.6h22.42v-5.6h5.6V61.65h-5.6v-5.61H72.84v-5.6h28.02v-5.6h5.6V33.63h-5.6v-5.61zM67.25 56.04v5.61h-5.6v5.6H44.84v-5.6h-5.6V44.84h5.6v-5.6h16.81v5.6h5.6v11.21zm89.89-28.02h-11.21v11.21h11.21zm33.63 11.21h11.21V28.02h-33.63v11.21z"/><path d="M179.56 72.86h-11.21V39.23h-11.21v56.05h-11.21v11.21h33.63V95.28h-11.21V84.07h33.63V72.86zm22.42-22.42v22.42h11.21V39.23h-11.21zm33.63-22.42H224.4v11.21h11.21v33.63H224.4v11.21h33.63V72.86h-11.21V39.23h11.21V28.02h-11.21V16.81h-11.21z"/><path d="M246.82 5.6v11.21h22.42V5.6zm56.05 56.05V5.6h-22.42v11.21h11.21v56.05h-11.21v11.21h33.63V72.86h-11.21zm33.63-11.21V39.23h-11.21v33.63h11.21zm22.42 0h-11.21v11.21h11.21zm0-11.21h11.21V28.02H336.5v11.21zm-11.21 33.63H336.5v11.21h33.63V72.86zm22.42-22.42v22.42h11.21V39.23h-11.21zm44.84-11.21V28.02h-22.42v11.21h11.21v22.42h11.21zm11.21 22.42h-11.21v11.21h11.21zm11.21 11.21h-11.21v11.21h11.21zm11.21-22.42V28.02h-11.21v44.84h11.21zm11.21 22.42H448.6v11.21h11.21zm11.21-11.21h-11.21v11.21h11.21zm11.21-33.63h-11.21v33.63h11.21V39.23h11.21V28.02z"/></g></svg>
  </a>
  <a class="pflow-home" href="/">&#9664; Home</a>
  <div id="pflow-loading">Loading model...</div>
  <div id="pflow-error">
    <h2>Model not found</h2>
    <p id="pflow-error-msg"></p>
    <p><a href="/pflow">View available models</a></p>
  </div>
  <script>
  // Hide Save/Share/My Diagrams menu items when hamburger menu opens
  (function() {
    var hideLabels = ['save', 'share', 'my diagrams', 'save as', 'login', 'sign in'];
    var observer = new MutationObserver(function() {
      var items = document.querySelectorAll('.pv-menu-item, .pv-hamburger-panel button, [class*="menu-item"]');
      for (var i = 0; i < items.length; i++) {
        var txt = (items[i].textContent || '').trim().toLowerCase();
        for (var j = 0; j < hideLabels.length; j++) {
          if (txt === hideLabels[j]) {
            items[i].style.display = 'none';
            // Also hide adjacent separators
            var next = items[i].nextElementSibling;
            if (next && (next.classList.contains('pv-menu-separator') || next.classList.contains('pv-hamburger-separator'))) {
              next.style.display = 'none';
            }
            break;
          }
        }
      }
    });
    observer.observe(document.body, { childList: true, subtree: true });
  })();
  </script>
  <script>
  (function() {
    var params = new URLSearchParams(window.location.search);
    var modelName = params.get('model');

    function escHtml(s) {
      if (!s) return '';
      return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
    }

    function generateThumbSVG(model) {
      var places = model.places || [];
      var transitions = model.transitions || [];
      var arcs = model.arcs || [];

      var nodePos = {};
      var nodeType = {};
      for (var i = 0; i < places.length; i++) nodeType[places[i].id] = 'place';
      for (var i = 0; i < transitions.length; i++) nodeType[transitions[i].id] = 'transition';

      // Check for explicit positions
      var hasPositions = false;
      for (var i = 0; i < places.length; i++) {
        if (places[i].x !== undefined || places[i].y !== undefined) { hasPositions = true; break; }
      }
      if (!hasPositions) {
        for (var i = 0; i < transitions.length; i++) {
          if (transitions[i].x !== undefined || transitions[i].y !== undefined) { hasPositions = true; break; }
        }
      }

      if (hasPositions) {
        for (var i = 0; i < places.length; i++) {
          var p = places[i];
          nodePos[p.id] = [p.x !== undefined ? p.x : 50 + i * 120, p.y !== undefined ? p.y : 50];
        }
        for (var i = 0; i < transitions.length; i++) {
          var t = transitions[i];
          nodePos[t.id] = [t.x !== undefined ? t.x : 50 + i * 120, t.y !== undefined ? t.y : 150];
        }
      } else {
        // Layered layout
        var outEdges = {}, inEdges = {}, allIds = [];
        for (var i = 0; i < places.length; i++) { allIds.push(places[i].id); outEdges[places[i].id] = []; inEdges[places[i].id] = []; }
        for (var i = 0; i < transitions.length; i++) { allIds.push(transitions[i].id); outEdges[transitions[i].id] = []; inEdges[transitions[i].id] = []; }
        for (var i = 0; i < arcs.length; i++) {
          if (outEdges[arcs[i].from]) outEdges[arcs[i].from].push(arcs[i].to);
          if (inEdges[arcs[i].to]) inEdges[arcs[i].to].push(arcs[i].from);
        }
        var layer = {}, visited = {};
        function assignLayer(id) {
          if (visited[id]) return layer[id] || 0;
          visited[id] = true;
          var maxPrev = -1;
          var ins = inEdges[id] || [];
          for (var j = 0; j < ins.length; j++) {
            var pl = assignLayer(ins[j]);
            if (pl > maxPrev) maxPrev = pl;
          }
          layer[id] = maxPrev + 1;
          return layer[id];
        }
        for (var i = 0; i < allIds.length; i++) assignLayer(allIds[i]);
        var layers = {}, maxLayer = 0;
        for (var i = 0; i < allIds.length; i++) {
          var l = layer[allIds[i]] || 0;
          if (!layers[l]) layers[l] = [];
          layers[l].push(allIds[i]);
          if (l > maxLayer) maxLayer = l;
        }
        var colSpacing = 120, rowSpacing = 80, padX = 60, padY = 50;
        for (var l = 0; l <= maxLayer; l++) {
          var nodes = layers[l] || [];
          for (var n = 0; n < nodes.length; n++) {
            nodePos[nodes[n]] = [padX + l * colSpacing, padY + n * rowSpacing];
          }
        }
      }

      // Compute bounds
      var maxX = 0, maxY = 0;
      for (var id in nodePos) {
        if (nodePos[id][0] > maxX) maxX = nodePos[id][0];
        if (nodePos[id][1] > maxY) maxY = nodePos[id][1];
      }
      var width = maxX + 100;
      var height = maxY + 80;

      var svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ' + width + ' ' + height + '" style="width:100%;height:100%;">';
      svg += '<defs><marker id="th-arrow" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">';
      svg += '<polygon points="0 0, 10 3.5, 0 7" fill="#8bb4d6"/></marker></defs>';
      svg += '<style>.th-place{fill:rgba(30,80,120,0.5);stroke:#4a9eda;stroke-width:2}.th-trans{fill:#c0c0c0;stroke:#c0c0c0}.th-label{font-family:system-ui,sans-serif;font-size:10px;text-anchor:middle;fill:#ccc}.th-arc{stroke:#8bb4d6;stroke-width:1.5;fill:none;marker-end:url(#th-arrow)}</style>';

      // Arcs
      for (var i = 0; i < arcs.length; i++) {
        var arc = arcs[i];
        var from = nodePos[arc.from];
        var to = nodePos[arc.to];
        if (!from || !to) continue;
        var dx = to[0] - from[0], dy = to[1] - from[1];
        var dist = Math.sqrt(dx * dx + dy * dy);
        if (dist < 1) continue;
        var ux = dx / dist, uy = dy / dist;
        var r1 = nodeType[arc.from] === 'place' ? 22 : 18;
        var r2 = nodeType[arc.to] === 'place' ? 27 : 20;
        var x1 = from[0] + ux * r1, y1 = from[1] + uy * r1;
        var x2 = to[0] - ux * r2, y2 = to[1] - uy * r2;
        svg += '<path d="M' + x1.toFixed(1) + ',' + y1.toFixed(1) + ' L' + x2.toFixed(1) + ',' + y2.toFixed(1) + '" class="th-arc"/>';
      }

      // Places
      for (var i = 0; i < places.length; i++) {
        var p = places[i];
        var pos = nodePos[p.id];
        if (!pos) continue;
        svg += '<circle cx="' + pos[0] + '" cy="' + pos[1] + '" r="20" class="th-place"/>';
        svg += '<text x="' + pos[0] + '" y="' + (pos[1] + 35) + '" class="th-label">' + escHtml(p.id) + '</text>';
        if (p.initial > 0) {
          svg += '<text x="' + pos[0] + '" y="' + (pos[1] + 4) + '" class="th-label" style="font-weight:bold;fill:#fff">' + p.initial + '</text>';
        }
      }

      // Transitions (rounded square, pflow style)
      for (var i = 0; i < transitions.length; i++) {
        var t = transitions[i];
        var pos = nodePos[t.id];
        if (!pos) continue;
        svg += '<rect x="' + (pos[0] - 12) + '" y="' + (pos[1] - 12) + '" width="24" height="24" rx="4" class="th-trans"/>';
        svg += '<text x="' + pos[0] + '" y="' + (pos[1] + 30) + '" class="th-label">' + escHtml(t.id) + '</text>';
      }

      svg += '</svg>';
      return svg;
    }

    if (!modelName) {
      // Show model picker with SVG thumbnails
      document.getElementById('pflow-loading').textContent = 'Loading models...';

      // Create empty petri-view so the full pflow UI renders behind
      var pv = document.createElement('petri-view');
      var pvScript = document.createElement('script');
      pvScript.type = 'application/ld+json';
      pvScript.textContent = JSON.stringify({
        '@context': 'https://pflow.xyz/schema',
        '@type': 'PetriNet',
        places: {}, transitions: {}, arcs: []
      });
      pv.appendChild(pvScript);
      document.body.appendChild(pv);

      fetch('/models')
        .then(function(r) { return r.json(); })
        .then(function(names) {
          document.getElementById('pflow-loading').style.display = 'none';
          // Modal overlay
          var overlay = document.createElement('div');
          overlay.style.cssText = 'position:fixed;top:0;left:0;right:0;bottom:0;z-index:900;background:rgba(0,0,0,0.65);display:flex;align-items:flex-start;justify-content:center;overflow-y:auto;padding:60px 20px 20px;';
          document.body.appendChild(overlay);
          var container = document.createElement('div');
          container.style.cssText = 'max-width:900px;width:100%;padding:32px;font-family:system-ui,sans-serif;color:#e0e4e8;background:#1a2535;border-radius:16px;border:1px solid rgba(255,255,255,0.1);box-shadow:0 20px 60px rgba(0,0,0,0.5);';
          overlay.appendChild(container);
          container.innerHTML = '<h1 style="font-size:24px;margin:0 0 8px;">Petri Net Viewer</h1>' +
            '<p style="color:#8899aa;margin:0 0 24px;">Select a model to view:</p>' +
            '<div id="pflow-grid" style="display:grid;grid-template-columns:repeat(auto-fill,minmax(240px,1fr));gap:16px;"></div>';
          var grid = document.getElementById('pflow-grid');
          for (var i = 0; i < names.length; i++) {
            (function(name) {
              var card = document.createElement('a');
              card.href = '/pflow?model=' + encodeURIComponent(name);
              card.style.cssText = 'display:block;border:1px solid rgba(255,255,255,0.1);border-radius:10px;text-decoration:none;color:#e0e4e8;overflow:hidden;transition:box-shadow 0.2s,transform 0.2s;background:#1e2a3a;';
              card.addEventListener('mouseenter', function() { this.style.boxShadow = '0 4px 20px rgba(0,0,0,0.4)'; this.style.transform = 'translateY(-2px)'; this.style.borderColor = 'rgba(255,255,255,0.2)'; });
              card.addEventListener('mouseleave', function() { this.style.boxShadow = ''; this.style.transform = ''; this.style.borderColor = 'rgba(255,255,255,0.1)'; });
              var thumb = document.createElement('div');
              thumb.style.cssText = 'height:160px;background:#253545;display:flex;align-items:center;justify-content:center;overflow:hidden;border-bottom:1px solid rgba(255,255,255,0.08);';
              thumb.innerHTML = '<div style="color:#aaa;font-size:13px;">Loading...</div>';
              card.appendChild(thumb);
              var info = document.createElement('div');
              info.style.cssText = 'padding:12px 14px;';
              info.innerHTML = '<div style="font-size:15px;font-weight:600;">' + escHtml(name) + '</div>';
              card.appendChild(info);
              grid.appendChild(card);
              // Fetch model and render thumbnail SVG
              fetch('/' + encodeURIComponent(name) + '/api/schema')
                .then(function(r) { return r.json(); })
                .then(function(model) {
                  thumb.innerHTML = generateThumbSVG(model);
                  // Add description if available
                  if (model.description) {
                    var desc = document.createElement('div');
                    desc.style.cssText = 'font-size:12px;color:#8899aa;margin-top:4px;line-height:1.4;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;';
                    desc.textContent = model.description;
                    info.appendChild(desc);
                  }
                  // Add stats
                  var places = (model.places || []).length;
                  var transitions = (model.transitions || []).length;
                  var stats = document.createElement('div');
                  stats.style.cssText = 'font-size:11px;color:#667788;margin-top:6px;';
                  stats.textContent = places + ' places \u00b7 ' + transitions + ' transitions';
                  info.appendChild(stats);
                })
                .catch(function() {
                  thumb.innerHTML = '<div style="color:#ccc;font-size:13px;">Preview unavailable</div>';
                });
            })(names[i]);
          }
        })
        .catch(function() {
          document.getElementById('pflow-loading').textContent = 'Failed to load models';
        });
      return;
    }

    // Fetch the Petri net model and convert to pflow JSON-LD format
    fetch('/' + encodeURIComponent(modelName) + '/api/schema')
      .then(function(r) {
        if (!r.ok) throw new Error('HTTP ' + r.status);
        return r.json();
      })
      .then(function(model) {
        document.getElementById('pflow-loading').style.display = 'none';
        document.title = (model.name || modelName) + ' - Petri Net Viewer';

        // Convert to pflow JSON-LD format
        var jsonLd = convertToPflowFormat(model);

        // Create petri-view element
        var pv = document.createElement('petri-view');
        var script = document.createElement('script');
        script.type = 'application/ld+json';
        script.textContent = JSON.stringify(jsonLd, null, 2);
        pv.appendChild(script);
        document.body.appendChild(pv);
      })
      .catch(function(err) {
        document.getElementById('pflow-loading').style.display = 'none';
        document.getElementById('pflow-error').style.display = 'flex';
        document.getElementById('pflow-error-msg').textContent = 'Could not load model "' + modelName + '": ' + err.message;
      });

    function convertToPflowFormat(model) {
      var places = model.places || [];
      var transitions = model.transitions || [];
      var arcs = model.arcs || [];

      // Build set of place and transition IDs for arc conversion
      var placeSet = {};
      var transSet = {};
      for (var i = 0; i < places.length; i++) placeSet[places[i].id] = true;
      for (var i = 0; i < transitions.length; i++) transSet[transitions[i].id] = true;

      // Auto-layout if no positions
      var hasPositions = false;
      for (var i = 0; i < places.length; i++) {
        if (places[i].x !== undefined || places[i].y !== undefined) { hasPositions = true; break; }
      }
      if (!hasPositions) {
        for (var i = 0; i < transitions.length; i++) {
          if (transitions[i].x !== undefined || transitions[i].y !== undefined) { hasPositions = true; break; }
        }
      }

      if (!hasPositions) {
        // Simple layered layout
        var outEdges = {}, inEdges = {}, allIds = [];
        for (var i = 0; i < places.length; i++) { allIds.push(places[i].id); outEdges[places[i].id] = []; inEdges[places[i].id] = []; }
        for (var i = 0; i < transitions.length; i++) { allIds.push(transitions[i].id); outEdges[transitions[i].id] = []; inEdges[transitions[i].id] = []; }
        for (var i = 0; i < arcs.length; i++) {
          if (outEdges[arcs[i].from]) outEdges[arcs[i].from].push(arcs[i].to);
          if (inEdges[arcs[i].to]) inEdges[arcs[i].to].push(arcs[i].from);
        }
        var layer = {}, visited = {};
        function assignLayer(id) {
          if (visited[id]) return layer[id] || 0;
          visited[id] = true;
          var maxPrev = -1;
          var ins = inEdges[id] || [];
          for (var j = 0; j < ins.length; j++) {
            var pl = assignLayer(ins[j]);
            if (pl > maxPrev) maxPrev = pl;
          }
          layer[id] = maxPrev + 1;
          return layer[id];
        }
        for (var i = 0; i < allIds.length; i++) assignLayer(allIds[i]);
        var layers = {}, maxLayer = 0;
        for (var i = 0; i < allIds.length; i++) {
          var l = layer[allIds[i]] || 0;
          if (!layers[l]) layers[l] = [];
          layers[l].push(allIds[i]);
          if (l > maxLayer) maxLayer = l;
        }
        var colSpacing = 160, rowSpacing = 120, padX = 80, padY = 80;
        for (var l = 0; l <= maxLayer; l++) {
          var nodes = layers[l] || [];
          for (var n = 0; n < nodes.length; n++) {
            var nid = nodes[n];
            var x = padX + l * colSpacing;
            var y = padY + n * rowSpacing;
            // Apply to original arrays
            for (var pi = 0; pi < places.length; pi++) {
              if (places[pi].id === nid) { places[pi].x = x; places[pi].y = y; }
            }
            for (var ti = 0; ti < transitions.length; ti++) {
              if (transitions[ti].id === nid) { transitions[ti].x = x; transitions[ti].y = y; }
            }
          }
        }
      }

      // Build pflow places object
      var pflowPlaces = {};
      for (var i = 0; i < places.length; i++) {
        var p = places[i];
        pflowPlaces[p.id] = {
          '@type': 'Place',
          x: p.x || 0,
          y: p.y || 0,
          initial: [typeof p.initial === 'number' ? p.initial : 0],
          capacity: [typeof p.initial === 'number' && p.initial > 0 ? p.initial : 1],
          offset: 0
        };
      }

      // Build pflow transitions object
      var pflowTransitions = {};
      for (var i = 0; i < transitions.length; i++) {
        var t = transitions[i];
        pflowTransitions[t.id] = {
          '@type': 'Transition',
          x: t.x || 0,
          y: t.y || 0
        };
      }

      // Build pflow arcs array
      var pflowArcs = [];
      for (var i = 0; i < arcs.length; i++) {
        var a = arcs[i];
        pflowArcs.push({
          '@type': 'Arrow',
          source: a.from,
          target: a.to,
          weight: [typeof a.weight === 'number' ? a.weight : 1],
          inhibitTransition: false
        });
      }

      return {
        '@context': 'https://pflow.xyz/schema',
        '@type': 'PetriNet',
        '@version': '1.1',
        places: pflowPlaces,
        transitions: pflowTransitions,
        arcs: pflowArcs,
        token: ['https://pflow.xyz/tokens/black']
      };
    }
  })();
  </script>
</body>
</html>`

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(pflowHTML))
	}
}
