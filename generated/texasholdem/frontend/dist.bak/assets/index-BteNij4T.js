var qe=Object.defineProperty;var We=(e,t,n)=>t in e?qe(e,t,{enumerable:!0,configurable:!0,writable:!0,value:n}):e[t]=n;var ne=(e,t,n)=>We(e,typeof t!="symbol"?t+"":t,n);(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const s of document.querySelectorAll('link[rel="modulepreload"]'))a(s);new MutationObserver(s=>{for(const i of s)if(i.type==="childList")for(const d of i.addedNodes)d.tagName==="LINK"&&d.rel==="modulepreload"&&a(d)}).observe(document,{childList:!0,subtree:!0});function n(s){const i={};return s.integrity&&(i.integrity=s.integrity),s.referrerPolicy&&(i.referrerPolicy=s.referrerPolicy),s.crossOrigin==="use-credentials"?i.credentials="include":s.crossOrigin==="anonymous"?i.credentials="omit":i.credentials="same-origin",i}function a(s){if(s.ep)return;s.ep=!0;const i=n(s);fetch(s.href,i)}})();const _e="texasholdem";class k extends HTMLElement{static get observedAttributes(){return[]}constructor(){super(),this.attachShadow({mode:"open"}),this._state={},this._props={},this._connected=!1}connectedCallback(){var t;this._connected=!0,this._initAttributes(),this.render(),(t=this.onConnect)==null||t.call(this)}disconnectedCallback(){var t;this._connected=!1,(t=this.onDisconnect)==null||t.call(this)}attributeChangedCallback(t,n,a){var s;n!==a&&(this._props[t]=this._parseAttribute(t,a),(s=this.onAttributeChange)==null||s.call(this,t,n,a),this._connected&&this.render())}_initAttributes(){const t=this.constructor.attributes||{};for(const[n,a]of Object.entries(t)){const s=this.getAttribute(n);this._props[n]=s!==null?this._parseAttribute(n,s):a.default}}_parseAttribute(t,n){const a=(this.constructor.attributes||{})[t];if(!a)return n;switch(a.type){case"number":return parseFloat(n)||a.default||0;case"boolean":return n!==null&&n!=="false";case"json":try{return JSON.parse(n)}catch{return a.default||null}default:return n}}prop(t){var n,a;return this._props[t]??((a=(n=this.constructor.attributes)==null?void 0:n[t])==null?void 0:a.default)}setProp(t,n){this._props[t]=n;const a=(this.constructor.attributes||{})[t];(a==null?void 0:a.type)==="json"?this.setAttribute(t,JSON.stringify(n)):(a==null?void 0:a.type)==="boolean"?n?this.setAttribute(t,""):this.removeAttribute(t):this.setAttribute(t,String(n))}set state(t){this._state=t,this._connected&&this.render()}get state(){return this._state}styles(){return""}template(){return""}render(){var t;this.shadowRoot.innerHTML=`
      <style>${this.baseStyles()}${this.styles()}</style>
      ${this.template()}
    `,(t=this.onRender)==null||t.call(this)}baseStyles(){return`
      :host { display: block; }
      :host([hidden]) { display: none; }
      * { box-sizing: border-box; }
      .card {
        background: white;
        border-radius: 8px;
        box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        padding: 1.5rem;
      }
      .btn {
        padding: 0.5rem 1rem;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9rem;
        font-weight: 500;
        transition: background 0.2s;
      }
      .btn-primary { background: #0066cc; color: white; }
      .btn-primary:hover { background: #0055aa; }
      .btn-success { background: #28a745; color: white; }
      .btn-success:hover { background: #218838; }
      .badge {
        display: inline-block;
        padding: 0.25rem 0.5rem;
        border-radius: 4px;
        font-size: 0.75rem;
        font-weight: 600;
      }
    `}emit(t,n={}){this.dispatchEvent(new CustomEvent(t,{detail:n,bubbles:!0,composed:!0}))}$(t){return this.shadowRoot.querySelector(t)}$$(t){return this.shadowRoot.querySelectorAll(t)}}ne(k,"attributes",{});const Pe=new Map;function _(e,t,{force:n=!1}={}){const a=e.includes("-")?e:`${_e}-${e}`;if(customElements.get(a)&&!n)return console.log(`[petri] Component ${a} already registered (extension override)`),!1;Pe.set(a,t);try{return customElements.define(a,t),!0}catch(s){return console.warn(`[petri] Could not register ${a}:`,s.message),!1}}function Ge(e){const t=e.includes("-")?e:`${_e}-${e}`;return Pe.get(t)||customElements.get(t)}class Ue extends k{static get observedAttributes(){return["loading"]}styles(){return`
      :host {
        display: block;
        padding: 2rem;
        max-width: 1200px;
        margin: 0 auto;
      }
      .dashboard-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 1.5rem;
      }
      .dashboard-header h1 {
        font-size: 1.5rem;
        font-weight: 600;
        margin: 0;
      }
      .dashboard-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
        gap: 1.5rem;
      }
      .panel {
        background: white;
        border-radius: 8px;
        box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        padding: 1.5rem;
      }
      .panel-title {
        font-size: 1rem;
        font-weight: 600;
        margin-bottom: 1rem;
        padding-bottom: 0.5rem;
        border-bottom: 1px solid #eee;
      }
      ::slotted([slot="header"]) {
        margin-bottom: 1.5rem;
      }
    `}template(){const{instances:t=[],stats:n={}}=this._state,a=this.hasAttribute("loading");return`
      <div class="dashboard-header">
        <h1>${this.getAttribute("title")||"Dashboard"}</h1>
        <slot name="actions"></slot>
      </div>
      <slot name="header"></slot>
      ${a?'<div class="loading">Loading...</div>':""}
      <div class="dashboard-grid">
        <slot name="stats"></slot>
        <slot name="panels"></slot>
        <slot></slot>
      </div>
    `}}class Je extends k{static get observedAttributes(){return["compact"]}styles(){return`
      .state-grid {
        display: grid;
        gap: 0.75rem;
      }
      .state-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.5rem 0;
        border-bottom: 1px solid #f0f0f0;
      }
      .state-item:last-child {
        border-bottom: none;
      }
      .state-label {
        color: #666;
        font-size: 0.9rem;
      }
      .state-value {
        font-weight: 600;
        font-family: monospace;
      }
      .state-value.positive { color: #28a745; }
      .state-value.zero { color: #999; }
      :host([compact]) .state-item {
        padding: 0.25rem 0;
        font-size: 0.85rem;
      }
    `}template(){const t=this._state||{};return`
      <div class="state-grid">
        <slot name="before"></slot>
        ${Object.entries(t).filter(([a])=>!a.startsWith("_")).map(([a,s])=>{const i=a.replace(/_/g," "),d=s>0?"positive":s===0?"zero":"";return`
          <div class="state-item">
            <span class="state-label">${i}</span>
            <span class="state-value ${d}">${s}</span>
          </div>
        `}).join("")}
        <slot name="after"></slot>
        <slot></slot>
      </div>
    `}}class Ze extends k{static get observedAttributes(){return["action","disabled","variant","label"]}styles(){return`
      button {
        width: 100%;
        padding: 0.75rem 1rem;
        border: none;
        border-radius: 6px;
        cursor: pointer;
        font-size: 0.9rem;
        font-weight: 500;
        transition: all 0.2s;
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 0.5rem;
      }
      button:disabled {
        opacity: 0.5;
        cursor: not-allowed;
      }
      button.primary { background: #0066cc; color: white; }
      button.primary:hover:not(:disabled) { background: #0055aa; }
      button.success { background: #28a745; color: white; }
      button.success:hover:not(:disabled) { background: #218838; }
      button.warning { background: #ffc107; color: #333; }
      button.warning:hover:not(:disabled) { background: #e0a800; }
      button.danger { background: #dc3545; color: white; }
      button.danger:hover:not(:disabled) { background: #c82333; }
      ::slotted(svg) { width: 1rem; height: 1rem; }
    `}template(){const t=this.getAttribute("action")||"",n=this.getAttribute("label")||t.replace(/_/g," "),a=this.getAttribute("variant")||"primary",s=this.hasAttribute("disabled");return`
      <button class="${a}" ${s?"disabled":""}>
        <slot name="icon"></slot>
        ${n}
      </button>
    `}onRender(){var t;(t=this.$("button"))==null||t.addEventListener("click",()=>{this.hasAttribute("disabled")||this.emit("action",{action:this.getAttribute("action")})})}}class Ye extends k{styles(){return`
      .card {
        background: white;
        border-radius: 8px;
        box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        padding: 1rem 1.5rem;
        cursor: pointer;
        transition: box-shadow 0.2s, transform 0.2s;
      }
      .card:hover {
        box-shadow: 0 4px 12px rgba(0,0,0,0.15);
        transform: translateY(-2px);
      }
      .card-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 0.5rem;
      }
      .card-id {
        font-family: monospace;
        font-size: 0.85rem;
        color: #666;
      }
      .card-state {
        font-size: 0.9rem;
        color: #333;
      }
      ::slotted([slot="badge"]) {
        margin-left: 0.5rem;
      }
    `}template(){const{id:t,state:n={}}=this._state,a=Object.entries(n).filter(([s,i])=>i>0&&!s.includes("_")).map(([s])=>s).join(", ")||"unknown";return`
      <div class="card">
        <div class="card-header">
          <span class="card-id">${(t==null?void 0:t.slice(0,8))||"..."}</span>
          <slot name="badge"></slot>
        </div>
        <div class="card-state">
          <slot name="state">${a}</slot>
        </div>
        <slot></slot>
      </div>
    `}onRender(){var t;(t=this.$(".card"))==null||t.addEventListener("click",()=>{this.emit("select",{instance:this._state})})}}class Se extends k{static get observedAttributes(){return["label","value","capacity","unit","warning-threshold","color"]}styles(){return`
      .gauge {
        padding: 1rem;
      }
      .gauge-header {
        display: flex;
        justify-content: space-between;
        margin-bottom: 0.5rem;
      }
      .gauge-label {
        font-weight: 500;
        color: #333;
      }
      .gauge-value {
        font-family: monospace;
        color: #666;
      }
      .gauge-bar {
        height: 8px;
        background: #e0e0e0;
        border-radius: 4px;
        overflow: hidden;
      }
      .gauge-fill {
        height: 100%;
        border-radius: 4px;
        transition: width 0.3s, background 0.3s;
      }
      .gauge-fill.good { background: #28a745; }
      .gauge-fill.warning { background: #ffc107; }
      .gauge-fill.danger { background: #dc3545; }
    `}template(){const t=this.prop("label"),n=this.prop("value"),a=this.prop("capacity"),s=this.prop("unit"),i=this.prop("warning-threshold"),d=this.prop("color"),r=a>0?n/a*100:0,u=d?"":r<i*100?"danger":r<50?"warning":"good",l=d?`background: ${d}`:"";return`
      <div class="gauge">
        <div class="gauge-header">
          <span class="gauge-label">${t}</span>
          <span class="gauge-value">${n}${s?` ${s}`:""} / ${a}</span>
        </div>
        <div class="gauge-bar">
          <div class="gauge-fill ${u}" style="width: ${r}%; ${l}"></div>
        </div>
        <slot></slot>
      </div>
    `}}ne(Se,"attributes",{label:{type:"string",default:"Resource"},value:{type:"number",default:0},capacity:{type:"number",default:100},unit:{type:"string",default:""},"warning-threshold":{type:"number",default:.3},color:{type:"string",default:""}});class Ke extends k{styles(){return`
      .flow-container {
        display: flex;
        gap: 1rem;
        overflow-x: auto;
        padding-bottom: 0.5rem;
      }
      .flow-column {
        flex: 1;
        min-width: 200px;
        background: #f8f9fa;
        border-radius: 8px;
        padding: 1rem;
      }
      .column-header {
        font-weight: 600;
        margin-bottom: 0.75rem;
        padding-bottom: 0.5rem;
        border-bottom: 2px solid #dee2e6;
        display: flex;
        justify-content: space-between;
        align-items: center;
      }
      .column-count {
        background: #6c757d;
        color: white;
        padding: 0.125rem 0.5rem;
        border-radius: 10px;
        font-size: 0.75rem;
      }
      .flow-items {
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
        min-height: 100px;
      }
      ::slotted([slot="column"]) {
        flex: 1;
        min-width: 200px;
      }
    `}template(){const{columns:t=[]}=this._state;return`
      <div class="flow-container">
        ${t.map(a=>`
      <div class="flow-column">
        <div class="column-header">
          <span>${a.label}</span>
          <span class="column-count">${a.count||0}</span>
        </div>
        <div class="flow-items" data-column="${a.id}">
          ${(a.items||[]).map(s=>`
            <slot name="item-${s.id}"></slot>
          `).join("")}
        </div>
      </div>
    `).join("")}
        <slot name="column"></slot>
        <slot></slot>
      </div>
    `}}function Xe(){_("dashboard",Ue),_("state-display",Je),_("action-button",Ze),_("instance-card",Ye),_("inventory-gauge",Se),_("order-flow",Ke)}window.__PETRI_SKIP_AUTO_REGISTER__||Xe();window.PetriElement=k;window.registerComponent=_;window.getComponent=Ge;const F=window.API_BASE||"";function B(){const t=window.location.pathname.match(/^(\/app\/[^\/]+)\//);return t?t[1]:F}function he(e){const t=B();return t&&e.startsWith(t)?e.slice(t.length)||"/":F&&e.startsWith(F)?e.slice(F.length)||"/":e}const re=[{path:"/",component:"List",title:"texas-holdem"},{path:"/texas-holdem",component:"List",title:"texas-holdem"},{path:"/texas-holdem/new",component:"Form",title:"New texas-holdem"},{path:"/texas-holdem/:id",component:"Detail",title:"texas-holdem Detail"},{path:"/schema",component:"Schema",title:"Schema Viewer"}];let S=null,C={};function q(e){e=e||"/",e!=="/"&&e.endsWith("/")&&(e=e.slice(0,-1));for(const t of re){const n={};let a=t.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");a=a.replace(/:[^/]+/g,"([^/]+)");const s=new RegExp(`^${a}$`),i=e.match(s);if(i)return(t.path.match(/:[^/]+/g)||[]).map(r=>r.slice(1)).forEach((r,u)=>{n[r]=decodeURIComponent(i[u+1])}),{route:t,params:n}}return null}function T(e,t={}){e.startsWith("/")||(e="/"+e);const n=he(e),a=q(n);if(!a){console.warn(`No route found for path: ${e}, falling back to list`);const d="/texas-holdem",r=q(d);r&&(S=r.route,C=r.params,window.history.pushState(t,"",`${B()}${d}`),de());return}if(a.route.roles&&a.route.roles.length>0){const d=Qe();if(!d||!et(d,a.route.roles)){console.warn("Access denied:",e),T(`${B()}/texas-holdem`);return}}S=a.route,C=a.params;const s=B(),i=e.startsWith(s)?e:`${s}${n}`;window.history.pushState(t,"",i),de()}window.addEventListener("popstate",()=>{const e=he(window.location.pathname),t=q(e);t?(S=t.route,C=t.params,de()):T(`${B()}/texas-holdem`)});function Qe(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function et(e,t){return!e||!e.roles?!1:t.some(n=>e.roles.includes(n))}function de(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:S,params:C}}))}function tt(){return C}function Ce(){return S}function nt(){const e=he(window.location.pathname),t=q(e);t?(S=t.route,C=t.params):(S=re.find(n=>n.path==="/texas-holdem")||re[0],C={})}const x=window.API_BASE||"";function Te(){const t=window.location.pathname.match(/^(\/app\/[^\/]+)\//);return t?t[1]:x}const ye=[];function ce(){const e=Te();return{brand:"texas-holdem",items:[{label:"texas-holdem",path:`${e}/texas-holdem`,icon:""},{label:"New",path:`${e}/texas-holdem/new`,icon:"+"},{label:"Schema",path:`${e}/schema`,icon:"⚙"}]}}let v=null,ae=!1,L=null;async function Ie(){if(!ae){ae=!0;try{const e={},t=ze();t&&(e.Authorization=`Bearer ${t}`);const n=await fetch(`${x}/api/navigation`,{headers:e});n.ok?v=await n.json():v=ce()}catch{v=ce()}finally{ae=!1}}}async function at(){if(L!==null)return L;try{let e=await fetch("/auth/status");e.ok||(e=await fetch(`${x}/auth/status`)),e.ok?L=await e.json():L={github_enabled:!1}}catch{L={github_enabled:!1}}return L}function ot(){const t=window.location.pathname.match(/^\/app\/([^\/]+)\//);return t?"/"+t[1]+"/":x+"/texas-holdem"}function st(){return window.location.pathname.match(/^\/app\/[^\/]+\//)!==null}async function Le(){v||await Ie();const e=window.location.pathname,t=rt(),n=ce(),a=(v==null?void 0:v.items)||n.items,s=(v==null?void 0:v.brand)||n.brand,i=ot(),d=st()?"":`onclick="handleNavClick(event, '${x}/texas-holdem')"`;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="${i}" ${d}>
          ${s}
        </a>
      </div>
      <ul class="nav-menu">
        ${a.map(u=>{const l=Te(),b=u.path.startsWith("/")?u.path:`${l}${u.path}`;return`
            <li class="${e===b||b!=="/"&&b!==l&&e.startsWith(b)?"active":""}">
              <a href="${b}" onclick="handleNavClick(event, '${b}')">
                ${u.icon?`<span class="icon">${u.icon}</span>`:""}
                ${u.label}
              </a>
            </li>
          `}).join("")}
      </ul>
      <div class="nav-user">
        ${t?`
          <span class="user-name">${t.roles?t.roles.join(", "):t.login||t.name||"User"}</span>
          <button onclick="handleLogout()" class="btn btn-link" style="color: rgba(255,255,255,0.8);">Logout</button>
        `:`
          <button onclick="showLoginModal()" class="btn btn-primary btn-sm">Login</button>
        `}
      </div>
    </nav>
    ${it()}
  `}function it(){const e=`
    <a href="/auth/login" class="github-login-btn" id="github-login-btn" style="display: none;">
      <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
        <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
      </svg>
      Login with GitHub
    </a>
  `,t=`
    <div class="login-divider" id="login-divider" style="display: none;">
      <span>or</span>
    </div>
  `;if(ye.length===0)return`
      <div id="login-modal" class="modal" style="display: none;">
        <div class="modal-backdrop" onclick="hideLoginModal()"></div>
        <div class="modal-content">
          <div class="modal-header">
            <h3>Login</h3>
            <button onclick="hideLoginModal()" class="modal-close">&times;</button>
          </div>
          <div class="modal-body">
            ${e}
            ${t}
            <p id="guest-login-text">No roles configured. Click below to login as a guest user.</p>
            <button onclick="handleRoleLogin(['guest'])" class="btn btn-primary" style="width: 100%; margin-top: 1rem;">
              Login as Guest
            </button>
          </div>
        </div>
      </div>
      ${we()}
    `;const n=ye.map(a=>`
    <button onclick="handleRoleLogin(['${a.id}'])" class="role-button">
      <span class="role-name">${a.id}</span>
      ${a.description?`<span class="role-desc">${a.description}</span>`:""}
    </button>
  `).join("");return`
    <div id="login-modal" class="modal" style="display: none;">
      <div class="modal-backdrop" onclick="hideLoginModal()"></div>
      <div class="modal-content">
        <div class="modal-header">
          <h3>Login</h3>
          <button onclick="hideLoginModal()" class="modal-close">&times;</button>
        </div>
        <div class="modal-body">
          ${e}
          ${t}
          <p>Choose a role to login as:</p>
          <div class="role-list">
            ${n}
          </div>
        </div>
      </div>
    </div>
    ${we()}
  `}function we(){return`
    <style>
      .modal {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        z-index: 1000;
        display: flex;
        align-items: center;
        justify-content: center;
      }
      .modal-backdrop {
        position: absolute;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background: rgba(0, 0, 0, 0.5);
      }
      .modal-content {
        position: relative;
        background: white;
        border-radius: 8px;
        padding: 0;
        min-width: 320px;
        max-width: 90%;
        box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
      }
      .modal-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 1rem 1.5rem;
        border-bottom: 1px solid #eee;
      }
      .modal-header h3 {
        margin: 0;
        font-size: 1.25rem;
      }
      .modal-close {
        background: none;
        border: none;
        font-size: 1.5rem;
        cursor: pointer;
        color: #666;
        padding: 0;
        line-height: 1;
      }
      .modal-close:hover {
        color: #333;
      }
      .modal-body {
        padding: 1.5rem;
      }
      .modal-body p {
        margin: 0 0 1rem 0;
        color: #666;
      }
      .github-login-btn {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 0.5rem;
        width: 100%;
        padding: 0.75rem 1rem;
        background: #24292e;
        color: white;
        border: none;
        border-radius: 6px;
        font-size: 1rem;
        font-weight: 500;
        cursor: pointer;
        text-decoration: none;
        transition: background 0.2s;
      }
      .github-login-btn:hover {
        background: #1a1e22;
        color: white;
      }
      .github-login-btn svg {
        flex-shrink: 0;
      }
      .login-divider {
        display: flex;
        align-items: center;
        margin: 1.25rem 0;
        color: #999;
        font-size: 0.85rem;
      }
      .login-divider::before,
      .login-divider::after {
        content: '';
        flex: 1;
        height: 1px;
        background: #ddd;
      }
      .login-divider span {
        padding: 0 1rem;
      }
      .role-list {
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
      }
      .role-button {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        padding: 0.75rem 1rem;
        border: 1px solid #ddd;
        border-radius: 6px;
        background: #f9f9f9;
        cursor: pointer;
        transition: all 0.2s;
        text-align: left;
      }
      .role-button:hover {
        background: #007bff;
        border-color: #007bff;
        color: white;
      }
      .role-button:hover .role-desc {
        color: rgba(255, 255, 255, 0.8);
      }
      .role-name {
        font-weight: 600;
        font-size: 1rem;
      }
      .role-desc {
        font-size: 0.85rem;
        color: #666;
        margin-top: 0.25rem;
      }
    </style>
  `}window.showLoginModal=async function(){const e=document.getElementById("login-modal");if(e){e.style.display="flex";const t=await at(),n=document.getElementById("github-login-btn"),a=document.getElementById("login-divider");t.github_enabled&&(n&&(n.style.display="flex"),a&&(a.style.display="flex"))}};window.hideLoginModal=function(){const e=document.getElementById("login-modal");e&&(e.style.display="none")};window.handleRoleLogin=async function(e){try{const t=await fetch(`${x}/api/debug/login`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({login:"pilot-user",roles:e})});if(!t.ok)throw new Error("Login failed");const n=await t.json();localStorage.setItem("auth",JSON.stringify(n)),hideLoginModal(),v=null,window.dispatchEvent(new CustomEvent("auth-change")),await G()}catch(t){console.error("Login error:",t),alert("Login failed. Please try again.")}};window.handleNavClick=function(e,t){e.preventDefault(),T(t)};window.handleLogout=async function(){try{const e=ze();e&&await fetch(`${x}/auth/logout`,{method:"POST",headers:{Authorization:`Bearer ${e}`}})}catch(e){console.error("Logout error:",e)}localStorage.removeItem("auth"),v=null,window.dispatchEvent(new CustomEvent("auth-change")),await G(),T(`${x}/texas-holdem`)};function rt(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function ze(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).token}catch{return null}return null}async function G(){v=null,await Ie();const e=document.getElementById("nav");e&&(e.innerHTML=await Le())}window.addEventListener("auth-change",async()=>{await G()});window.addEventListener("route-change",()=>{const e=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(t=>{t.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(t=>{const n=t.getAttribute("href");(n===e||n!=="/"&&n!==(x||"")&&e.startsWith(n))&&t.parentElement.classList.add("active")})});const dt=window.API_BASE||"";let xe=[];async function ct(){try{const e=await fetch(`${dt}/api/views`);if(!e.ok)return[];const t=e.headers.get("content-type");return!t||!t.includes("application/json")?[]:(xe=await e.json(),xe)}catch{return[]}}function lt(){return customElements.get("texasholdem-dashboard")!==void 0}const P=window.API_BASE||"",ut="";function ht(e){return["amount","value","balance","total_supply","allowance"].some(n=>e.toLowerCase().includes(n))}let w=null,$=null,le=[],m=null;const U=[{id:"start_hand",name:"Start Hand",description:"Start a new hand",fields:[]},{id:"deal_preflop",name:"Deal Preflop",description:"Deal hole cards to all players",fields:[]},{id:"deal_flop",name:"Deal Flop",description:"Deal the flop (3 community cards)",fields:[]},{id:"deal_turn",name:"Deal Turn",description:"Deal the turn (1 community card)",fields:[]},{id:"deal_river",name:"Deal River",description:"Deal the river (1 community card)",fields:[]},{id:"go_showdown",name:"Go Showdown",description:"Proceed to showdown",fields:[]},{id:"determine_winner",name:"Determine Winner",description:"Determine the winner",fields:[]},{id:"end_hand",name:"End Hand",description:"End the hand",fields:[]},{id:"advance_to_p1",name:"Advance To P1",description:"Pass turn to player 1",fields:[]},{id:"advance_to_p2",name:"Advance To P2",description:"Pass turn to player 2",fields:[]},{id:"advance_to_p3",name:"Advance To P3",description:"Pass turn to player 3",fields:[]},{id:"advance_to_p4",name:"Advance To P4",description:"Pass turn to player 4",fields:[]},{id:"advance_to_p0",name:"Advance To P0",description:"Pass turn to player 0",fields:[]},{id:"end_betting_round",name:"End Betting Round",description:"End the betting round",fields:[]},{id:"p0_fold",name:"P0 Fold",description:"Player 0 folds",fields:[]},{id:"p0_check",name:"P0 Check",description:"Player 0 checks",fields:[]},{id:"p0_call",name:"P0 Call",description:"Player 0 calls",fields:[]},{id:"p0_raise",name:"P0 Raise",description:"Player 0 raises",fields:[]},{id:"p1_fold",name:"P1 Fold",description:"Player 1 folds",fields:[]},{id:"p1_check",name:"P1 Check",description:"Player 1 checks",fields:[]},{id:"p1_call",name:"P1 Call",description:"Player 1 calls",fields:[]},{id:"p1_raise",name:"P1 Raise",description:"Player 1 raises",fields:[]},{id:"p2_fold",name:"P2 Fold",description:"Player 2 folds",fields:[]},{id:"p2_check",name:"P2 Check",description:"Player 2 checks",fields:[]},{id:"p2_call",name:"P2 Call",description:"Player 2 calls",fields:[]},{id:"p2_raise",name:"P2 Raise",description:"Player 2 raises",fields:[]},{id:"p3_fold",name:"P3 Fold",description:"Player 3 folds",fields:[]},{id:"p3_check",name:"P3 Check",description:"Player 3 checks",fields:[]},{id:"p3_call",name:"P3 Call",description:"Player 3 calls",fields:[]},{id:"p3_raise",name:"P3 Raise",description:"Player 3 raises",fields:[]},{id:"p4_fold",name:"P4 Fold",description:"Player 4 folds",fields:[]},{id:"p4_check",name:"P4 Check",description:"Player 4 checks",fields:[]},{id:"p4_call",name:"P4 Call",description:"Player 4 calls",fields:[]},{id:"p4_raise",name:"P4 Raise",description:"Player 4 raises",fields:[]}];function mt(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);if(t.expires_at&&new Date(t.expires_at)>new Date)return $=t.token,w=t.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function Me(e){localStorage.setItem("auth",JSON.stringify(e)),$=e.token,w=e.user,window.dispatchEvent(new CustomEvent("auth-change"))}function J(){localStorage.removeItem("auth"),$=null,w=null,window.dispatchEvent(new CustomEvent("auth-change"))}function pt(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);return $=t.token,w=t.user,!0}catch{return!1}return $=null,w=null,!1}window.addEventListener("auth-change",()=>{pt()});function z(){const e={"Content-Type":"application/json"};return $&&(e.Authorization=`Bearer ${$}`),e}async function j(e){if(e.status===401)throw J(),O("Session expired. Please log in again."),new Error("Unauthorized");if(!e.ok){const t=await e.json().catch(()=>({}));throw new Error(t.message||e.statusText)}return e.json()}const I={async getMe(){const e=await fetch(`${P}/auth/me`,{headers:z()});return j(e)},async logout(){await fetch(`${P}/auth/logout`,{method:"POST",headers:z()}),J()},async listInstances(){const e=await fetch(`${P}/admin/instances`,{headers:z()});return j(e)},async getInstance(e){const t=await fetch(`${P}/api/texasholdem/${e}`,{headers:z()});return j(t)},async createInstance(e={}){const t=await fetch(`${P}/api/texasholdem`,{method:"POST",headers:z(),body:JSON.stringify(e)});return j(t)},async executeTransition(e,t,n={}){var r,u;const a=n,s=(u=(r=window.pilot)==null?void 0:r.getTransition)==null?void 0:u.call(r,e);let i=(s==null?void 0:s.apiPath)||`/api/${e}`;i=i.replace("{id}",t);const d=await fetch(`${P}${i}`,{method:"POST",headers:z(),body:JSON.stringify({aggregate_id:t,data:a})});return j(d)}};window.api=I;Object.defineProperty(window,"currentInstance",{get:function(){return m}});window.setAuthToken=function(e){$=e};window.saveAuth=Me;window.clearAuth=J;function O(e){const t=document.getElementById("app"),n=t.querySelector(".alert-error");n&&n.remove();const a=document.createElement("div");a.className="alert alert-error",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),5e3)}function me(e){const t=document.getElementById("app"),n=t.querySelector(".alert-success");n&&n.remove();const a=document.createElement("div");a.className="alert alert-success",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),3e3)}const $e={},ke="Unknown";function Ne(e){if(!e)return ke;for(const[t,n]of Object.entries(e))if(n>0&&$e[t])return $e[t];return ke}function je(e){return`<span class="badge ${`badge-${e.toLowerCase().replace(/_/g,"-")}`}">${e.replace(/_/g," ")}</span>`}async function oe(){const e=document.getElementById("app");if(lt()){e.innerHTML="<texasholdem-dashboard></texasholdem-dashboard>";return}e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>texas-holdem</h1>
        <button class="btn btn-primary" onclick="handleCreateNew()">+ New</button>
      </div>
      <div id="instances-list" class="entity-list">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{le=(await I.listInstances()).instances||[],ft()}catch{document.getElementById("instances-list").innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `}}function ft(){const e=document.getElementById("instances-list");if(e){if(le.length===0){e.innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `;return}e.innerHTML=le.map(t=>{const n=Ne(t.state||t.places);return`
      <div class="entity-card" onclick="navigate('/texas-holdem/${t.id}')">
        <div class="entity-info">
          <h3>${t.id}</h3>
          <div class="entity-meta">
            ${je(n)} &middot; Version ${t.version||0}
          </div>
        </div>
        <div class="entity-actions">
          <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); navigate('/texas-holdem/${t.id}')">
            View
          </button>
        </div>
      </div>
    `}).join("")}}async function gt(){const t=tt().id,n=document.getElementById("app");n.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/texas-holdem')" style="margin-left: -0.5rem">
            &larr; Back to List
          </button>
          <h1 style="margin-top: 0.5rem">Instance: ${t}</h1>
        </div>
      </div>
      <div id="instance-detail">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{const a=await I.getInstance(t);m={id:a.aggregate_id||t,version:a.version,state:a.state,displayState:a.state,places:a.places,enabled:a.enabled||a.enabled_transitions||[]},window.currentInstanceState=m.state,pe()}catch(a){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${a.message}</div>
    `}}function pe(){const e=document.getElementById("instance-detail");if(!e||!m)return;const t=Ne(m.places),n=m.enabled||[],a=U;e.innerHTML=`
    <div class="card">
      <div class="card-header">Status</div>
      <div class="detail-list">
        <div class="detail-field">
          <dt>ID</dt>
          <dd><code>${m.id}</code></dd>
        </div>
        <div class="detail-field">
          <dt>Status</dt>
          <dd>${je(t)}</dd>
        </div>
        <div class="detail-field">
          <dt>Version</dt>
          <dd>${m.version||0}</dd>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="card-header">Actions</div>
      <div class="view-actions">
        ${a.map(s=>{const i=n.includes(s.id);return`
            <button
              class="btn ${i?"btn-primary":"btn-secondary"}"
              onclick="handleTransition('${s.id}')"
              ${i?"":"disabled"}
              title="${s.description||s.name}"
            >
              ${s.name}
            </button>
          `}).join("")}
      </div>
      ${n.length===0?'<p style="color: #666; margin-top: 1rem;">No actions available in current state.</p>':""}
    </div>

    <div class="card">
      <div class="card-header">Current State</div>
      <div class="detail-list">
        ${bt(m.displayState||m.state)}
      </div>
    </div>
  `}function bt(e){return!e||Object.keys(e).length===0?'<p style="color: #999;">No state data</p>':Object.entries(e).map(([t,n])=>{if(typeof n=="object"&&n!==null){const a=Object.entries(n);return a.length===0?`
          <div class="detail-field">
            <dt>${se(t)}</dt>
            <dd><span style="color: #999;">Empty</span></dd>
          </div>
        `:`
        <div class="detail-field">
          <dt>${se(t)}</dt>
          <dd>
            <div class="nested-state">
              ${a.map(([s,i])=>{if(typeof i=="object"&&i!==null){const d=Object.entries(i);return d.length===0?`
                      <div class="state-entry">
                        <span class="state-key">${s}</span>
                        <span class="state-value" style="color: #999;">Empty</span>
                      </div>
                    `:`
                    <div class="state-entry nested-group">
                      <span class="state-key">${s}</span>
                      <div class="nested-state" style="margin-left: 1rem;">
                        ${d.map(([r,u])=>`
                          <div class="state-entry">
                            <span class="state-key">${r}</span>
                            <span class="state-value">${ie(t,u)}</span>
                          </div>
                        `).join("")}
                      </div>
                    </div>
                  `}return`
                  <div class="state-entry">
                    <span class="state-key">${s}</span>
                    <span class="state-value">${ie(t,i)}</span>
                  </div>
                `}).join("")}
            </div>
          </dd>
        </div>
      `}return`
      <div class="detail-field">
        <dt>${se(t)}</dt>
        <dd>${ie(t,n)}</dd>
      </div>
    `}).join("")}function se(e){return e.replace(/_/g," ").replace(/\b\w/g,t=>t.toUpperCase())}function ie(e,t){return ht(e),`<strong>${t}</strong>`}function vt(){if(typeof wallet<"u"&&wallet.getAccount){const e=wallet.getAccount();return(e==null?void 0:e.address)||null}return null}function yt(e,t){return e?e==="wallet"?vt()||"":e==="user"?(w==null?void 0:w.id)||(w==null?void 0:w.login)||"":(e.startsWith("balances.")||e.includes("."),""):""}function wt(){return`
    <div id="action-modal" class="modal" style="display: none;">
      <div class="modal-backdrop" onclick="hideActionModal()"></div>
      <div class="modal-content">
        <div class="modal-header">
          <h3 id="action-modal-title">Execute Action</h3>
          <button onclick="hideActionModal()" class="modal-close">&times;</button>
        </div>
        <div class="modal-body">
          <form id="action-form" onsubmit="handleActionSubmit(event)">
            <div id="action-form-fields"></div>
            <div class="form-actions">
              <button type="submit" class="btn btn-primary">Execute</button>
              <button type="button" class="btn btn-secondary" onclick="hideActionModal()">Cancel</button>
            </div>
          </form>
        </div>
      </div>
    </div>
    <style>
      .modal {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        z-index: 1000;
        display: flex;
        align-items: center;
        justify-content: center;
      }
      .modal-backdrop {
        position: absolute;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background: rgba(0, 0, 0, 0.5);
      }
      .modal-content {
        position: relative;
        background: white;
        border-radius: 8px;
        padding: 0;
        min-width: 400px;
        max-width: 90%;
        box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
      }
      .modal-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 1rem 1.5rem;
        border-bottom: 1px solid #eee;
      }
      .modal-header h3 {
        margin: 0;
        font-size: 1.25rem;
      }
      .modal-close {
        background: none;
        border: none;
        font-size: 1.5rem;
        cursor: pointer;
        color: #666;
        padding: 0;
        line-height: 1;
      }
      .modal-close:hover {
        color: #333;
      }
      .modal-body {
        padding: 1.5rem;
      }
      .address-input-wrapper {
        position: relative;
      }
      .address-picker-btn {
        position: absolute;
        right: 4px;
        top: 50%;
        transform: translateY(-50%);
        background: #f0f0f0;
        border: 1px solid #ddd;
        border-radius: 4px;
        padding: 4px 8px;
        font-size: 0.85rem;
        cursor: pointer;
      }
      .address-picker-btn:hover {
        background: #e0e0e0;
      }
      .field-description {
        font-size: 0.85rem;
        color: #666;
        margin-top: 0.25rem;
      }
    </style>
  `}let W=null;function xt(e){const t=U.find(a=>a.id===e);if(!t)return;W=e,document.getElementById("action-modal-title").textContent=t.name;const n=t.fields.map(a=>{const i=yt(a.autoFill,m==null?void 0:m.state)||a.defaultValue||"",d=a.required?"required":"";let r="";if(a.type==="amount")r=`
        <input
          type="number"
          name="${a.name}"
          value="${i}"
          placeholder="${a.placeholder||"Amount"}"
          step="any"
          ${d}
          class="form-control"
        />
        
      `;else if(a.type==="address"){const u=$t();u.length>0?r=`
          <select name="${a.name}" ${d} class="form-control">
            <option value="">Select address...</option>
            ${u.map(l=>`
              <option value="${l.address}" ${l.address===i?"selected":""}>
                ${l.name||"Account"} (${l.address.slice(0,8)}...${l.address.slice(-6)})
              </option>
            `).join("")}
          </select>
        `:r=`
          <input
            type="text"
            name="${a.name}"
            value="${i}"
            placeholder="${a.placeholder||"0x..."}"
            ${d}
            class="form-control"
          />
        `}else a.type==="hidden"?r=`<input type="hidden" name="${a.name}" value="${i}" />`:r=`
        <input
          type="${a.type==="number"?"number":"text"}"
          name="${a.name}"
          value="${i}"
          placeholder="${a.placeholder||""}"
          ${d}
          class="form-control"
        />
      `;return a.type==="hidden"?r:`
      <div class="form-field">
        <label>${a.label}${a.required?" *":""}</label>
        ${r}
      </div>
    `}).join("");document.getElementById("action-form-fields").innerHTML=n,document.getElementById("action-modal").style.display="flex"}window.hideActionModal=function(){document.getElementById("action-modal").style.display="none",W=null};window.handleActionSubmit=async function(e){var d;if(e.preventDefault(),!W||!m)return;const t=W,n=m.id,a=e.target,s=new FormData(a),i={};for(const[r,u]of s.entries()){const l=(d=U.find(b=>b.id===t))==null?void 0:d.fields.find(b=>b.name===r);l&&(l.type==="amount"||l.type==="number")?i[r]=parseFloat(u)||0:i[r]=u}hideActionModal();try{const r=await I.executeTransition(t,n,i);m={...m,version:r.version,state:r.state,displayState:r.state,places:r.state,enabled:r.enabled||[]},window.currentInstanceState=m.state,pe(),me(`Action "${t}" completed!`)}catch(r){O(`Failed to execute ${t}: ${r.message}`)}};function $t(){return typeof wallet<"u"&&wallet.getAccounts?wallet.getAccounts()||[]:[]}window.showAddressPicker=function(e){if(typeof wallet>"u"||!wallet.getAccounts)return;const t=wallet.getAccounts();if(!t||t.length===0)return;const n=document.querySelector(".address-picker-dropdown");n&&n.remove();const a=document.querySelector(`[name="${e}"]`);if(!a)return;const s=a.getBoundingClientRect(),i=document.createElement("div");i.className="address-picker-dropdown",i.style.cssText=`
    position: fixed;
    top: ${s.bottom+4}px;
    left: ${s.left}px;
    width: ${s.width}px;
    background: white;
    border: 1px solid #ddd;
    border-radius: 4px;
    box-shadow: 0 4px 12px rgba(0,0,0,0.15);
    z-index: 2000;
    max-height: 200px;
    overflow-y: auto;
  `,i.innerHTML=t.map(d=>`
    <div class="address-picker-option" onclick="selectAddress('${e}', '${d.address}')" style="
      padding: 8px 12px;
      cursor: pointer;
      border-bottom: 1px solid #eee;
    ">
      <div style="font-weight: 500;">${d.name||"Account"}</div>
      <div style="font-size: 0.85rem; color: #666; font-family: monospace;">${d.address.slice(0,10)}...${d.address.slice(-8)}</div>
    </div>
  `).join(""),document.body.appendChild(i),setTimeout(()=>{document.addEventListener("click",function d(r){i.contains(r.target)||(i.remove(),document.removeEventListener("click",d))})},0)};window.selectAddress=function(e,t){const n=document.querySelector(`[name="${e}"]`);n&&(n.value=t,n.dispatchEvent(new Event("input",{bubbles:!0})));const a=document.querySelector(".address-picker-dropdown");a&&a.remove()};async function kt(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/texas-holdem')" style="margin-left: -0.5rem">
            &larr; Cancel
          </button>
          <h1 style="margin-top: 0.5rem">Create New</h1>
        </div>
      </div>
      <div class="card">
        <form id="create-form" onsubmit="handleSubmitCreate(event)">
          <p style="color: #666; margin-bottom: 1rem;">Create a new workflow instance. The instance will start in the initial state.</p>
          <div class="form-actions">
            <button type="submit" class="btn btn-primary">Create</button>
            <button type="button" class="btn btn-secondary" onclick="navigate('/texas-holdem')">Cancel</button>
          </div>
        </form>
      </div>
    </div>
  `}async function Et(){var t,n,a,s,i,d,r;const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>Schema Viewer</h1>
        <p style="color: #666; margin-top: 0.5rem;">Inspect the Petri net model that powers this application</p>
      </div>
      <div id="schema-content" class="card">
        <div class="loading">Loading schema...</div>
      </div>
    </div>
  `;try{const l=await(await fetch(`${P}/api/schema`)).json();ue=l;const b=document.getElementById("schema-content");b.innerHTML=`
      <div class="schema-viewer">
        <div class="schema-tabs">
          <button class="schema-tab active" onclick="showSchemaTab('overview')">Overview</button>
          <button class="schema-tab" onclick="showSchemaTab('petrinet')">Petri Net</button>
          <button class="schema-tab" onclick="showSchemaTab('places')">Places (${((t=l.places)==null?void 0:t.length)||0})</button>
          <button class="schema-tab" onclick="showSchemaTab('transitions')">Transitions (${((n=l.transitions)==null?void 0:n.length)||0})</button>
          <button class="schema-tab" onclick="showSchemaTab('arcs')">Arcs (${((a=l.arcs)==null?void 0:a.length)||0})</button>
          <button class="schema-tab" onclick="showSchemaTab('raw')">Raw JSON</button>
        </div>

        <div id="schema-tab-overview" class="schema-tab-content active">
          <div class="schema-overview">
            <div class="schema-info-card">
              <h3>${l.name||"Unnamed"}</h3>
              <p>${l.description||"No description"}</p>
              <a class="pflow-link" href="/pflow?model=${encodeURIComponent(l.name||"")}" target="_blank">
                Open in <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 490 115"><g transform="translate(5,5)"><path d="M100.88 28.02H78.46v5.61h-5.6v5.6h-5.6v-5.6h5.6v-5.61h5.6V5.6h-5.6V0H61.65v5.6h-5.6v28.02h-5.6V5.6h-5.6V0H33.64v5.6h-5.6v22.42h5.6v5.61h5.6v5.6h-5.6v-5.6h-5.6v-5.61H5.6v5.61H0v11.21h5.6v5.6h28.02v5.6H5.6v5.61H0v11.21h5.6v5.6h22.42v-5.6h5.6v-5.61h5.6v5.61h-5.6v5.6h-5.6v22.42h5.6v5.6h11.21v-5.6h5.6V72.86h5.6v28.02h5.6v5.6h11.21v-5.6h5.6V78.46h-5.6v-5.6h-5.6v-5.61h5.6v5.61h5.6v5.6h22.42v-5.6h5.6V61.65h-5.6v-5.61H72.84v-5.6h28.02v-5.6h5.6V33.63h-5.6v-5.61zM67.25 56.04v5.61h-5.6v5.6H44.84v-5.6h-5.6V44.84h5.6v-5.6h16.81v5.6h5.6v11.21zm89.89-28.02h-11.21v11.21h11.21zm33.63 11.21h11.21V28.02h-33.63v11.21z"/><path d="M179.56 72.86h-11.21V39.23h-11.21v56.05h-11.21v11.21h33.63V95.28h-11.21V84.07h33.63V72.86zm22.42-22.42v22.42h11.21V39.23h-11.21zm33.63-22.42H224.4v11.21h11.21v33.63H224.4v11.21h33.63V72.86h-11.21V39.23h11.21V28.02h-11.21V16.81h-11.21z"/><path d="M246.82 5.6v11.21h22.42V5.6zm56.05 56.05V5.6h-22.42v11.21h11.21v56.05h-11.21v11.21h33.63V72.86h-11.21zm33.63-11.21V39.23h-11.21v33.63h11.21zm22.42 0h-11.21v11.21h11.21zm0-11.21h11.21V28.02H336.5v11.21zm-11.21 33.63H336.5v11.21h33.63V72.86zm22.42-22.42v22.42h11.21V39.23h-11.21zm44.84-11.21V28.02h-22.42v11.21h11.21v22.42h11.21zm11.21 22.42h-11.21v11.21h11.21zm11.21 11.21h-11.21v11.21h11.21zm11.21-22.42V28.02h-11.21v44.84h11.21zm11.21 22.42H448.6v11.21h11.21zm11.21-11.21h-11.21v11.21h11.21zm11.21-33.63h-11.21v33.63h11.21V39.23h11.21V28.02z"/></g></svg>
              </a>
            </div>
            <div class="schema-stats">
              <div class="stat-item">
                <span class="stat-value">${((s=l.places)==null?void 0:s.length)||0}</span>
                <span class="stat-label">Places</span>
              </div>
              <div class="stat-item">
                <span class="stat-value">${((i=l.transitions)==null?void 0:i.length)||0}</span>
                <span class="stat-label">Transitions</span>
              </div>
              <div class="stat-item">
                <span class="stat-value">${((d=l.arcs)==null?void 0:d.length)||0}</span>
                <span class="stat-label">Arcs</span>
              </div>
              <div class="stat-item">
                <span class="stat-value">${((r=l.roles)==null?void 0:r.length)||0}</span>
                <span class="stat-label">Roles</span>
              </div>
            </div>
          </div>
        </div>

        <div id="schema-tab-places" class="schema-tab-content" style="display: none;">
          <table class="schema-table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Description</th>
                <th>Initial</th>
                <th>Capacity</th>
              </tr>
            </thead>
            <tbody>
              ${(l.places||[]).map(p=>`
                <tr>
                  <td><code>${p.id}</code></td>
                  <td>${p.description||"-"}</td>
                  <td>${p.initial||0}</td>
                  <td>${p.capacity||"∞"}</td>
                </tr>
              `).join("")}
            </tbody>
          </table>
        </div>

        <div id="schema-tab-transitions" class="schema-tab-content" style="display: none;">
          <table class="schema-table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Description</th>
                <th>Guard</th>
                <th>Roles</th>
              </tr>
            </thead>
            <tbody>
              ${(l.transitions||[]).map(p=>`
                <tr>
                  <td><code>${p.id}</code></td>
                  <td>${p.description||"-"}</td>
                  <td><code>${p.guard||"-"}</code></td>
                  <td>${(p.roles||[]).join(", ")||"-"}</td>
                </tr>
              `).join("")}
            </tbody>
          </table>
        </div>

        <div id="schema-tab-arcs" class="schema-tab-content" style="display: none;">
          <table class="schema-table">
            <thead>
              <tr>
                <th>From</th>
                <th>To</th>
                <th>Weight</th>
                <th>Type</th>
              </tr>
            </thead>
            <tbody>
              ${(l.arcs||[]).map(p=>`
                <tr>
                  <td><code>${p.from}</code></td>
                  <td><code>${p.to}</code></td>
                  <td>${p.weight||1}</td>
                  <td>${p.type||"normal"}</td>
                </tr>
              `).join("")}
            </tbody>
          </table>
        </div>

        <div id="schema-tab-petrinet" class="schema-tab-content" style="display: none;">
          <div class="petrinet-container">
            <petri-view id="schema-petri-view"></petri-view>
          </div>
        </div>

        <div id="schema-tab-raw" class="schema-tab-content" style="display: none;">
          <pre class="schema-json">${JSON.stringify(l,null,2)}</pre>
        </div>
      </div>

      <style>
        .schema-viewer {
          padding: 1rem;
        }
        .schema-tabs {
          display: flex;
          gap: 0.5rem;
          margin-bottom: 1rem;
          border-bottom: 1px solid #eee;
          padding-bottom: 0.5rem;
        }
        .schema-tab {
          padding: 0.5rem 1rem;
          border: none;
          background: #f5f5f5;
          border-radius: 4px;
          cursor: pointer;
          font-size: 0.9rem;
        }
        .schema-tab:hover {
          background: #e9e9e9;
        }
        .schema-tab.active {
          background: #007bff;
          color: white;
        }
        .schema-tab-content {
          display: none;
        }
        .schema-tab-content.active {
          display: block;
        }
        .schema-overview {
          display: flex;
          flex-direction: column;
          gap: 1.5rem;
        }
        .schema-info-card h3 {
          margin: 0 0 0.5rem 0;
          font-size: 1.5rem;
        }
        .schema-info-card p {
          margin: 0;
          color: #666;
        }
        .pflow-link {
          display: inline-flex;
          align-items: center;
          gap: 6px;
          margin-top: 0.75rem;
          padding: 6px 14px;
          background: #f5f5f5;
          border: 1px solid #ddd;
          border-radius: 6px;
          text-decoration: none;
          color: #555;
          font-size: 0.85rem;
          transition: background 0.15s, border-color 0.15s;
        }
        .pflow-link:hover {
          background: #e9e9e9;
          border-color: #bbb;
          color: #333;
        }
        .pflow-link svg {
          height: 16px;
          width: auto;
          fill: currentColor;
          vertical-align: -2px;
        }
        .schema-stats {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
          gap: 1rem;
        }
        .stat-item {
          background: #f8f9fa;
          padding: 1rem;
          border-radius: 8px;
          text-align: center;
        }
        .stat-value {
          display: block;
          font-size: 2rem;
          font-weight: 600;
          color: #007bff;
        }
        .stat-label {
          display: block;
          font-size: 0.85rem;
          color: #666;
          margin-top: 0.25rem;
        }
        .schema-table {
          width: 100%;
          border-collapse: collapse;
        }
        .schema-table th,
        .schema-table td {
          padding: 0.75rem;
          text-align: left;
          border-bottom: 1px solid #eee;
        }
        .schema-table th {
          background: #f8f9fa;
          font-weight: 600;
        }
        .schema-table code {
          background: #f0f0f0;
          padding: 0.2rem 0.4rem;
          border-radius: 3px;
          font-size: 0.85rem;
        }
        .schema-json {
          background: #1e1e1e;
          color: #d4d4d4;
          padding: 1rem;
          border-radius: 8px;
          overflow-x: auto;
          font-size: 0.85rem;
          line-height: 1.5;
        }
        .petrinet-container {
          width: 100%;
          height: calc(100vh - 280px);
          min-height: 500px;
          border: 1px solid #ddd;
          border-radius: 8px;
          overflow: hidden;
        }
        .petrinet-container petri-view {
          width: 100%;
          height: 100%;
        }
      </style>
    `}catch(u){console.error("Failed to load schema:",u),document.getElementById("schema-content").innerHTML=`
      <div class="error">Failed to load schema: ${u.message}</div>
    `}}let ue=null;function At(e){const t={},n={},a=[],s=new Set((e.places||[]).map(o=>o.id)),i=new Set((e.transitions||[]).map(o=>o.id)),d={},r={},u={},l={};(e.places||[]).forEach(o=>{d[o.id]=[],u[o.id]=[]}),(e.transitions||[]).forEach(o=>{r[o.id]=[],l[o.id]=[]}),(e.arcs||[]).forEach(o=>{s.has(o.from)&&i.has(o.to)?(d[o.from].push(o.to),l[o.to].push(o.from)):i.has(o.from)&&s.has(o.to)&&(r[o.from].push(o.to),u[o.to].push(o.from))});const b=[],p=new Set,R=[];for((e.places||[]).forEach(o=>{((o.initial||0)>0||u[o.id].length===0)&&(R.push(o.id),p.add(o.id))});R.length>0;){const o=R.shift();b.push(o),d[o].forEach(h=>{r[h].forEach(c=>{p.has(c)||(p.add(c),R.push(c))})})}(e.places||[]).forEach(o=>{p.has(o.id)||b.push(o.id)});function fe(o){if(["in_review","in_progress","create_post","can_reset","game_active"].includes(o))return o;let c=o.match(/^([a-zA-Z]+_[a-zA-Z]+)_\d/);return c||(c=o.match(/^([a-zA-Z]+)\d/),c)||(c=o.match(/^([a-zA-Z]+_[a-zA-Z]+)_[a-zA-Z]+\d/),c)||(c=o.match(/^([a-zA-Z]+_[a-zA-Z]+)_[a-zA-Z]+$/),c)||(c=o.match(/^([a-zA-Z]+)_[a-zA-Z]$/),c)||(c=o.match(/^([a-zA-Z]+)_([a-zA-Z]+)$/),c&&c[2].length<=4)?c[1]:o}const V={};b.forEach((o,h)=>{const c=fe(o);V[c]||(V[c]={places:[],minOrder:h}),V[c].places.push({id:o,order:h})});const Be=Object.entries(V).sort((o,h)=>o[1].minOrder-h[1].minOrder).map(([o,h])=>({prefix:o,...h})),Z=80,H=100,ge=130,be=120,Oe=80,Y={},Re={};(e.places||[]).forEach(o=>{Re[o.id]=o});let K=Z,X=H;Be.forEach((o,h)=>{const c=o.places.sort((E,A)=>{var N,D;const ee=parseInt(((N=E.id.match(/\d+$/))==null?void 0:N[0])||"0"),te=parseInt(((D=A.id.match(/\d+$/))==null?void 0:D[0])||"0");return ee-te}),f=c.length;let g,y;f<=4?(g=f,y=1):f===9?(g=3,y=3):f<=9?(g=Math.ceil(Math.sqrt(f)),y=Math.ceil(f/g)):(g=Math.min(f,6),y=Math.ceil(f/g)),c.forEach((E,A)=>{const ee=A%g,te=Math.floor(A/g),N=H+ee*ge,D=K+te*be;Y[E.id]={x:N,y:D},X=Math.max(X,N)}),K+=y*be+Oe});let Ve=K;(e.places||[]).forEach(o=>{const c=o.x!==void 0&&o.y!==void 0&&(o.x!==0||o.y!==0)?{x:o.x,y:o.y}:Y[o.id]||{x:H,y:Z},f=o.initial||0,g=o.capacity!==void 0?o.capacity:null;t[o.id]={"@type":"Place",initial:[f],capacity:[g],x:c.x,y:c.y}});const Q={};(e.transitions||[]).forEach(o=>{const h=fe(o.id);Q[h]||(Q[h]=[]),Q[h].push(o)});const ve={},He=X+ge+80,De=100,M={};(e.transitions||[]).forEach(o=>{const h=l[o.id]||[];let c=Z;if(h.length>0){let g=0,y=0;h.forEach(E=>{const A=Y[E];A&&(g+=A.y,y++)}),y>0&&(c=Math.round(g/y))}const f=Math.round(c/80)*80;M[f]||(M[f]=[]),M[f].push({t:o,primaryY:c})}),Object.keys(M).map(Number).sort((o,h)=>o-h).forEach((o,h)=>{const c=M[o];c.sort((f,g)=>f.t.id.localeCompare(g.t.id)),c.forEach((f,g)=>{const y=He+g*De,E=f.primaryY;ve[f.t.id]={x:y,y:E}})}),(e.transitions||[]).forEach(o=>{const c=o.x!==void 0&&o.y!==void 0&&(o.x!==0||o.y!==0)?{x:o.x,y:o.y}:ve[o.id]||{x:H,y:Ve+150};n[o.id]={"@type":"Transition",x:c.x,y:c.y}});const Fe=(e.arcs||[]).every(o=>(o.weight||1)===1);return(e.arcs||[]).forEach(o=>{const h={"@type":"Arrow",source:o.from,target:o.to,inhibit:o.type==="inhibitor"};Fe||(h.weight=[o.weight||1]),a.push(h)}),{"@context":"https://pflow.xyz/schema","@type":"PetriNet",name:e.name||"Model",description:e.description||"",places:t,transitions:n,arcs:a}}let Ee=!1;async function _t(){if(!Ee){if(!document.querySelector('link[href*="petri-view.css"]')){const e=document.createElement("link");e.rel="stylesheet",e.href="https://cdn.jsdelivr.net/gh/pflow-xyz/pflow-xyz@main/public/petri-view.css",document.head.appendChild(e)}customElements.get("petri-view")||(await new Promise((e,t)=>{const n=document.createElement("script");n.type="module",n.src="https://cdn.jsdelivr.net/gh/pflow-xyz/pflow-xyz@main/public/petri-view.js",n.onload=e,n.onerror=t,document.head.appendChild(n)}),await customElements.whenDefined("petri-view")),Ee=!0}}async function Pt(){if(!ue)return;await _t();const e=document.getElementById("schema-petri-view");if(!e)return;await(async(n=20)=>{for(let a=0;a<n;a++){if(e.setModel){const s=At(ue);e.setModel(s),await new Promise(i=>requestAnimationFrame(i)),e._toggleSim&&!e._simRunning&&e._toggleSim(),St();return}await new Promise(s=>setTimeout(s,50))}console.warn("petri-view setModel not available after waiting")})()}function St(e){if(document.getElementById("petri-toolbar-override"))return;const t=document.createElement("style");t.id="petri-toolbar-override",t.textContent=`
    /* Move petri-view toolbar from bottom to top of canvas, aligned with hamburger menu */
    .pv-menu.pv-mode-menu {
      bottom: auto !important;
      top: 260px !important;
    }
  `,document.head.appendChild(t)}window.showSchemaTab=function(e){document.querySelectorAll(".schema-tab").forEach(n=>{n.classList.remove("active")}),event.target.classList.add("active"),document.querySelectorAll(".schema-tab-content").forEach(n=>{n.style.display="none",n.classList.remove("active")});const t=document.getElementById(`schema-tab-${e}`);t&&(t.style.display="block",t.classList.add("active")),e==="petrinet"?Pt():(document.body.style.overflow="auto",document.documentElement.style.overflow="auto")};window.navigate=T;window.handleCreateNew=async function(){T("/texas-holdem/new")};window.handleSubmitCreate=async function(e){e.preventDefault();try{const t=await I.createInstance({});me("Instance created successfully!"),T(`/texas-holdem/${t.aggregate_id||t.id}`)}catch(t){O("Failed to create: "+t.message)}};window.handleTransition=async function(e){if(!m)return;const t=U.find(n=>n.id===e);if(t&&t.fields&&t.fields.length>0){xt(e);return}try{const n=await I.executeTransition(e,m.id);m={...m,version:n.version,state:n.state,displayState:n.state,places:n.state,enabled:n.enabled||[]},window.currentInstanceState=m.state,pe(),me(`Action "${e}" completed!`)}catch(n){O(`Failed to execute ${e}: ${n.message}`)}};function Ae(e){var a;const t=((a=e.detail)==null?void 0:a.route)||Ce();if(!t){oe();return}const n=t.path;n==="/texas-holdem"||n==="/"?oe():n==="/texas-holdem/new"?kt():n==="/texas-holdem/:id"?gt():n==="/schema"?Et():oe()}async function Ct(){const e=new URLSearchParams(window.location.search),t=e.get("token"),n=e.get("expires_at");if(t){$=t;try{const a=await I.getMe();Me({token:t,expires_at:n,user:a}),window.history.replaceState({},"",window.location.pathname),await G()}catch{J(),O("Failed to complete login")}}}async function Tt(){mt(),await Ct(),await ct();const e=document.getElementById("nav");e.innerHTML=await Le();const t=document.createElement("div");t.innerHTML=wt(),document.body.appendChild(t),window.addEventListener("route-change",Ae),nt(),Ae({detail:{route:Ce()}})}Tt();
