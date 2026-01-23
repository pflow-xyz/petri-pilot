var ae=Object.defineProperty;var oe=(e,t,n)=>t in e?ae(e,t,{enumerable:!0,configurable:!0,writable:!0,value:n}):e[t]=n;var P=(e,t,n)=>oe(e,typeof t!="symbol"?t+"":t,n);(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const a of document.querySelectorAll('link[rel="modulepreload"]'))s(a);new MutationObserver(a=>{for(const o of a)if(o.type==="childList")for(const c of o.addedNodes)c.tagName==="LINK"&&c.rel==="modulepreload"&&s(c)}).observe(document,{childList:!0,subtree:!0});function n(a){const o={};return a.integrity&&(o.integrity=a.integrity),a.referrerPolicy&&(o.referrerPolicy=a.referrerPolicy),a.crossOrigin==="use-credentials"?o.credentials="include":a.crossOrigin==="anonymous"?o.credentials="omit":o.credentials="same-origin",o}function s(a){if(a.ep)return;a.ep=!0;const o=n(a);fetch(a.href,o)}})();const Q="testaccess";class $ extends HTMLElement{static get observedAttributes(){return[]}constructor(){super(),this.attachShadow({mode:"open"}),this._state={},this._props={},this._connected=!1}connectedCallback(){var t;this._connected=!0,this._initAttributes(),this.render(),(t=this.onConnect)==null||t.call(this)}disconnectedCallback(){var t;this._connected=!1,(t=this.onDisconnect)==null||t.call(this)}attributeChangedCallback(t,n,s){var a;n!==s&&(this._props[t]=this._parseAttribute(t,s),(a=this.onAttributeChange)==null||a.call(this,t,n,s),this._connected&&this.render())}_initAttributes(){const t=this.constructor.attributes||{};for(const[n,s]of Object.entries(t)){const a=this.getAttribute(n);this._props[n]=a!==null?this._parseAttribute(n,a):s.default}}_parseAttribute(t,n){const s=(this.constructor.attributes||{})[t];if(!s)return n;switch(s.type){case"number":return parseFloat(n)||s.default||0;case"boolean":return n!==null&&n!=="false";case"json":try{return JSON.parse(n)}catch{return s.default||null}default:return n}}prop(t){var n,s;return this._props[t]??((s=(n=this.constructor.attributes)==null?void 0:n[t])==null?void 0:s.default)}setProp(t,n){this._props[t]=n;const s=(this.constructor.attributes||{})[t];(s==null?void 0:s.type)==="json"?this.setAttribute(t,JSON.stringify(n)):(s==null?void 0:s.type)==="boolean"?n?this.setAttribute(t,""):this.removeAttribute(t):this.setAttribute(t,String(n))}set state(t){this._state=t,this._connected&&this.render()}get state(){return this._state}styles(){return""}template(){return""}render(){var t;this.shadowRoot.innerHTML=`
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
    `}emit(t,n={}){this.dispatchEvent(new CustomEvent(t,{detail:n,bubbles:!0,composed:!0}))}$(t){return this.shadowRoot.querySelector(t)}$$(t){return this.shadowRoot.querySelectorAll(t)}}P($,"attributes",{});const X=new Map;function x(e,t,{force:n=!1}={}){const s=e.includes("-")?e:`${Q}-${e}`;if(customElements.get(s)&&!n)return console.log(`[petri] Component ${s} already registered (extension override)`),!1;X.set(s,t);try{return customElements.define(s,t),!0}catch(a){return console.warn(`[petri] Could not register ${s}:`,a.message),!1}}function ie(e){const t=e.includes("-")?e:`${Q}-${e}`;return X.get(t)||customElements.get(t)}class re extends ${static get observedAttributes(){return["loading"]}styles(){return`
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
    `}template(){const{instances:t=[],stats:n={}}=this._state,s=this.hasAttribute("loading");return`
      <div class="dashboard-header">
        <h1>${this.getAttribute("title")||"Dashboard"}</h1>
        <slot name="actions"></slot>
      </div>
      <slot name="header"></slot>
      ${s?'<div class="loading">Loading...</div>':""}
      <div class="dashboard-grid">
        <slot name="stats"></slot>
        <slot name="panels"></slot>
        <slot></slot>
      </div>
    `}}class ce extends ${static get observedAttributes(){return["compact"]}styles(){return`
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
        ${Object.entries(t).filter(([s])=>!s.startsWith("_")).map(([s,a])=>{const o=s.replace(/_/g," "),c=a>0?"positive":a===0?"zero":"";return`
          <div class="state-item">
            <span class="state-label">${o}</span>
            <span class="state-value ${c}">${a}</span>
          </div>
        `}).join("")}
        <slot name="after"></slot>
        <slot></slot>
      </div>
    `}}class de extends ${static get observedAttributes(){return["action","disabled","variant","label"]}styles(){return`
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
    `}template(){const t=this.getAttribute("action")||"",n=this.getAttribute("label")||t.replace(/_/g," "),s=this.getAttribute("variant")||"primary",a=this.hasAttribute("disabled");return`
      <button class="${s}" ${a?"disabled":""}>
        <slot name="icon"></slot>
        ${n}
      </button>
    `}onRender(){var t;(t=this.$("button"))==null||t.addEventListener("click",()=>{this.hasAttribute("disabled")||this.emit("action",{action:this.getAttribute("action")})})}}class le extends ${styles(){return`
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
    `}template(){const{id:t,state:n={}}=this._state,s=Object.entries(n).filter(([a,o])=>o>0&&!a.includes("_")).map(([a])=>a).join(", ")||"unknown";return`
      <div class="card">
        <div class="card-header">
          <span class="card-id">${(t==null?void 0:t.slice(0,8))||"..."}</span>
          <slot name="badge"></slot>
        </div>
        <div class="card-state">
          <slot name="state">${s}</slot>
        </div>
        <slot></slot>
      </div>
    `}onRender(){var t;(t=this.$(".card"))==null||t.addEventListener("click",()=>{this.emit("select",{instance:this._state})})}}class Z extends ${static get observedAttributes(){return["label","value","capacity","unit","warning-threshold","color"]}styles(){return`
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
    `}template(){const t=this.prop("label"),n=this.prop("value"),s=this.prop("capacity"),a=this.prop("unit"),o=this.prop("warning-threshold"),c=this.prop("color"),r=s>0?n/s*100:0,l=c?"":r<o*100?"danger":r<50?"warning":"good",d=c?`background: ${c}`:"";return`
      <div class="gauge">
        <div class="gauge-header">
          <span class="gauge-label">${t}</span>
          <span class="gauge-value">${n}${a?` ${a}`:""} / ${s}</span>
        </div>
        <div class="gauge-bar">
          <div class="gauge-fill ${l}" style="width: ${r}%; ${d}"></div>
        </div>
        <slot></slot>
      </div>
    `}}P(Z,"attributes",{label:{type:"string",default:"Resource"},value:{type:"number",default:0},capacity:{type:"number",default:100},unit:{type:"string",default:""},"warning-threshold":{type:"number",default:.3},color:{type:"string",default:""}});class ue extends ${styles(){return`
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
        ${t.map(s=>`
      <div class="flow-column">
        <div class="column-header">
          <span>${s.label}</span>
          <span class="column-count">${s.count||0}</span>
        </div>
        <div class="flow-items" data-column="${s.id}">
          ${(s.items||[]).map(a=>`
            <slot name="item-${a.id}"></slot>
          `).join("")}
        </div>
      </div>
    `).join("")}
        <slot name="column"></slot>
        <slot></slot>
      </div>
    `}}function me(){x("dashboard",re),x("state-display",ce),x("action-button",de),x("instance-card",le),x("inventory-gauge",Z),x("order-flow",ue)}window.__PETRI_SKIP_AUTO_REGISTER__||me();window.PetriElement=$;window.registerComponent=x;window.getComponent=ie;const z=[{path:"/",component:"List",title:"test-access"},{path:"/test-access",component:"List",title:"test-access"},{path:"/test-access/new",component:"Form",title:"New test-access"},{path:"/test-access/:id",component:"Detail",title:"test-access Detail"},{path:"/schema",component:"Schema",title:"Schema Viewer"},{path:"/admin",component:"AdminDashboard",title:"Admin Dashboard"},{path:"/admin/instances",component:"AdminInstances",title:"Instances"},{path:"/admin/instances/:id",component:"AdminInstance",title:"Instance Detail"}];let E=null,k={};function N(e){e=e||"/",e!=="/"&&e.endsWith("/")&&(e=e.slice(0,-1));for(const t of z){const n={};let s=t.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");s=s.replace(/:[^/]+/g,"([^/]+)");const a=new RegExp(`^${s}$`),o=e.match(a);if(o)return(t.path.match(/:[^/]+/g)||[]).map(r=>r.slice(1)).forEach((r,l)=>{n[r]=decodeURIComponent(o[l+1])}),{route:t,params:n}}return null}function h(e,t={}){e.startsWith("/")||(e="/"+e);const n=N(e);if(!n){console.warn(`No route found for path: ${e}, falling back to list`),e="/test-access";const s=N(e);s&&(E=s.route,k=s.params,window.history.pushState(t,"",e),H());return}if(n.route.roles&&n.route.roles.length>0){const s=pe();if(!s||!he(s,n.route.roles)){console.warn("Access denied:",e),h("/test-access");return}}E=n.route,k=n.params,window.history.pushState(t,"",e),H()}window.addEventListener("popstate",()=>{const e=window.location.pathname,t=N(e);t?(E=t.route,k=t.params,H()):h("/test-access")});function pe(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function he(e,t){return!e||!e.roles?!1:t.some(n=>e.roles.includes(n))}function H(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:E,params:k}}))}function ge(){return k}function L(){return E}function be(){const e=window.location.pathname,t=N(e);t?(E=t.route,k=t.params):(E=z.find(n=>n.path==="/test-access")||z[0],k={})}const K=[{id:"customer",description:"Regular customer who can submit items"},{id:"reviewer",description:"Can review and approve/reject submissions"},{id:"admin",description:"Full access to all operations"}],I={brand:"test-access",items:[{label:"test-access",path:"/test-access",icon:""},{label:"New",path:"/test-access/new",icon:"+"},{label:"Schema",path:"/schema",icon:"⚙"},{label:"Admin",path:"/admin",icon:""}]};let p=null,D=!1;async function ee(){if(!D){D=!0;try{const e={},t=ne();t&&(e.Authorization=`Bearer ${t}`);const n=await fetch("/api/navigation",{headers:e});n.ok?p=await n.json():p=I}catch{p=I}finally{D=!1}}}async function te(){p||await ee();const e=window.location.pathname,t=ve(),n=(p==null?void 0:p.items)||I.items;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="/test-access" onclick="handleNavClick(event, '/test-access')">
          ${(p==null?void 0:p.brand)||I.brand}
        </a>
      </div>
      <ul class="nav-menu">
        ${n.map(o=>`
            <li class="${e===o.path||o.path!=="/"&&e.startsWith(o.path)?"active":""}">
              <a href="${o.path}" onclick="handleNavClick(event, '${o.path}')">
                ${o.icon?`<span class="icon">${o.icon}</span>`:""}
                ${o.label}
              </a>
            </li>
          `).join("")}
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
    ${fe()}
  `}function fe(){return K.length===0?`
      <div id="login-modal" class="modal" style="display: none;">
        <div class="modal-backdrop" onclick="hideLoginModal()"></div>
        <div class="modal-content">
          <div class="modal-header">
            <h3>Login</h3>
            <button onclick="hideLoginModal()" class="modal-close">&times;</button>
          </div>
          <div class="modal-body">
            <p>No roles configured. Click below to login as a guest user.</p>
            <button onclick="handleRoleLogin(['guest'])" class="btn btn-primary" style="width: 100%; margin-top: 1rem;">
              Login as Guest
            </button>
          </div>
        </div>
      </div>
    `:`
    <div id="login-modal" class="modal" style="display: none;">
      <div class="modal-backdrop" onclick="hideLoginModal()"></div>
      <div class="modal-content">
        <div class="modal-header">
          <h3>Select Role</h3>
          <button onclick="hideLoginModal()" class="modal-close">&times;</button>
        </div>
        <div class="modal-body">
          <p>Choose a role to login as:</p>
          <div class="role-list">
            ${K.map(t=>`
    <button onclick="handleRoleLogin(['${t.id}'])" class="role-button">
      <span class="role-name">${t.id}</span>
      ${t.description?`<span class="role-desc">${t.description}</span>`:""}
    </button>
  `).join("")}
          </div>
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
  `}window.showLoginModal=function(){const e=document.getElementById("login-modal");e&&(e.style.display="flex")};window.hideLoginModal=function(){const e=document.getElementById("login-modal");e&&(e.style.display="none")};window.handleRoleLogin=async function(e){try{const t=await fetch("/api/debug/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({login:"pilot-user",roles:e})});if(!t.ok)throw new Error("Login failed");const n=await t.json();localStorage.setItem("auth",JSON.stringify(n)),hideLoginModal(),p=null,window.dispatchEvent(new CustomEvent("auth-change")),await O()}catch(t){console.error("Login error:",t),alert("Login failed. Please try again.")}};window.handleNavClick=function(e,t){e.preventDefault(),h(t)};window.handleLogout=async function(){try{const e=ne();e&&await fetch("/auth/logout",{method:"POST",headers:{Authorization:`Bearer ${e}`}})}catch(e){console.error("Logout error:",e)}localStorage.removeItem("auth"),p=null,window.dispatchEvent(new CustomEvent("auth-change")),await O(),h("/test-access")};function ve(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function ne(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).token}catch{return null}return null}async function O(){p=null,await ee();const e=document.getElementById("nav");e&&(e.innerHTML=await te())}window.addEventListener("auth-change",async()=>{await O()});window.addEventListener("route-change",()=>{const e=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(t=>{t.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(t=>{const n=t.getAttribute("href");(n===e||n!=="/"&&e.startsWith(n))&&t.parentElement.classList.add("active")})});let G=[];async function we(){try{const e=await fetch("/api/views");return e.ok?(G=await e.json(),G):(console.warn("Failed to load view definitions, using defaults"),[])}catch(e){return console.error("Error loading views:",e),[]}}const b="",ye="";function $e(e){return["amount","value","balance","total_supply","allowance"].some(n=>e.toLowerCase().includes(n))}let u=null,f=null,y=[],i=null;const F=[{id:"submit",name:"Submit",description:"Submit for review",fields:[]},{id:"approve",name:"Approve",description:"Approve the submission",fields:[]},{id:"reject",name:"Reject",description:"Reject the submission",fields:[]}];function xe(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);if(t.expires_at&&new Date(t.expires_at)>new Date)return f=t.token,u=t.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function J(e){localStorage.setItem("auth",JSON.stringify(e)),f=e.token,u=e.user,window.dispatchEvent(new CustomEvent("auth-change"))}function R(){localStorage.removeItem("auth"),f=null,u=null,window.dispatchEvent(new CustomEvent("auth-change"))}function Ee(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);return f=t.token,u=t.user,!0}catch{return!1}return f=null,u=null,!1}window.addEventListener("auth-change",()=>{Ee()});function v(){const e={"Content-Type":"application/json"};return f&&(e.Authorization=`Bearer ${f}`),e}async function S(e){if(e.status===401)throw R(),A("Session expired. Please log in again."),new Error("Unauthorized");if(!e.ok){const t=await e.json().catch(()=>({}));throw new Error(t.message||e.statusText)}return e.json()}const g={async getMe(){const e=await fetch(`${b}/auth/me`,{headers:v()});return S(e)},async logout(){await fetch(`${b}/auth/logout`,{method:"POST",headers:v()}),R()},async listInstances(){const e=await fetch(`${b}/admin/instances`,{headers:v()});return S(e)},async getInstance(e){const t=await fetch(`${b}/api/testaccess/${e}`,{headers:v()});return S(t)},async createInstance(e={}){const t=await fetch(`${b}/api/testaccess`,{method:"POST",headers:v(),body:JSON.stringify(e)});return S(t)},async executeTransition(e,t,n={}){var r,l;const s=n,a=(l=(r=window.pilot)==null?void 0:r.getTransition)==null?void 0:l.call(r,e);let o=(a==null?void 0:a.apiPath)||`/api/${e}`;o=o.replace("{id}",t);const c=await fetch(`${b}${o}`,{method:"POST",headers:v(),body:JSON.stringify({aggregate_id:t,data:s})});return S(c)}};window.api=g;Object.defineProperty(window,"currentInstance",{get:function(){return i}});window.setAuthToken=function(e){f=e};window.saveAuth=J;window.clearAuth=R;function A(e){const t=document.getElementById("app"),n=t.querySelector(".alert-error");n&&n.remove();const s=document.createElement("div");s.className="alert alert-error",s.textContent=e,t.insertBefore(s,t.firstChild),setTimeout(()=>s.remove(),5e3)}function W(e){const t=document.getElementById("app"),n=t.querySelector(".alert-success");n&&n.remove();const s=document.createElement("div");s.className="alert alert-success",s.textContent=e,t.insertBefore(s,t.firstChild),setTimeout(()=>s.remove(),3e3)}function U(e){if(!e)return"unknown";for(const[t,n]of Object.entries(e))if(n>0)return t;return"unknown"}function V(e){return`<span class="badge ${`badge-${e.toLowerCase().replace(/_/g,"-")}`}">${e.replace(/_/g," ")}</span>`}async function M(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>test-access</h1>
        <button class="btn btn-primary" onclick="handleCreateNew()">+ New</button>
      </div>
      <div id="instances-list" class="entity-list">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{y=(await g.listInstances()).instances||[],ke()}catch{document.getElementById("instances-list").innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `}}function ke(){const e=document.getElementById("instances-list");if(e){if(y.length===0){e.innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `;return}e.innerHTML=y.map(t=>{const n=U(t.state||t.places);return`
      <div class="entity-card" onclick="navigate('/test-access/${t.id}')">
        <div class="entity-info">
          <h3>${t.id}</h3>
          <div class="entity-meta">
            ${V(n)} &middot; Version ${t.version||0}
          </div>
        </div>
        <div class="entity-actions">
          <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); navigate('/test-access/${t.id}')">
            View
          </button>
        </div>
      </div>
    `}).join("")}}async function Se(){const t=ge().id,n=document.getElementById("app");n.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/test-access')" style="margin-left: -0.5rem">
            &larr; Back to List
          </button>
          <h1 style="margin-top: 0.5rem">Instance: ${t}</h1>
        </div>
      </div>
      <div id="instance-detail">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{const s=await g.getInstance(t);i={id:s.aggregate_id||t,version:s.version,state:s.state,displayState:s.state,places:s.places,enabled:s.enabled||s.enabled_transitions||[]},window.currentInstanceState=i.state,C()}catch(s){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${s.message}</div>
    `}}function C(){const e=document.getElementById("instance-detail");if(!e||!i)return;const t=U(i.places),n=i.enabled||[],s=F;e.innerHTML=`
    <div class="card">
      <div class="card-header">Status</div>
      <div class="detail-list">
        <div class="detail-field">
          <dt>ID</dt>
          <dd><code>${i.id}</code></dd>
        </div>
        <div class="detail-field">
          <dt>Status</dt>
          <dd>${V(t)}</dd>
        </div>
        <div class="detail-field">
          <dt>Version</dt>
          <dd>${i.version||0}</dd>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="card-header">Actions</div>
      <div class="view-actions">
        ${s.map(a=>{const o=n.includes(a.id);return`
            <button
              class="btn ${o?"btn-primary":"btn-secondary"}"
              onclick="handleTransition('${a.id}')"
              ${o?"":"disabled"}
              title="${a.description||a.name}"
            >
              ${a.name}
            </button>
          `}).join("")}
      </div>
      ${n.length===0?'<p style="color: #666; margin-top: 1rem;">No actions available in current state.</p>':""}
    </div>

    <div class="card">
      <div class="card-header">Current State</div>
      <div class="detail-list">
        ${Ae(i.displayState||i.state)}
      </div>
    </div>
  `}function Ae(e){return!e||Object.keys(e).length===0?'<p style="color: #999;">No state data</p>':Object.entries(e).map(([t,n])=>{if(typeof n=="object"&&n!==null){const s=Object.entries(n);return s.length===0?`
          <div class="detail-field">
            <dt>${q(t)}</dt>
            <dd><span style="color: #999;">Empty</span></dd>
          </div>
        `:`
        <div class="detail-field">
          <dt>${q(t)}</dt>
          <dd>
            <div class="nested-state">
              ${s.map(([a,o])=>{if(typeof o=="object"&&o!==null){const c=Object.entries(o);return c.length===0?`
                      <div class="state-entry">
                        <span class="state-key">${a}</span>
                        <span class="state-value" style="color: #999;">Empty</span>
                      </div>
                    `:`
                    <div class="state-entry nested-group">
                      <span class="state-key">${a}</span>
                      <div class="nested-state" style="margin-left: 1rem;">
                        ${c.map(([r,l])=>`
                          <div class="state-entry">
                            <span class="state-key">${r}</span>
                            <span class="state-value">${B(t,l)}</span>
                          </div>
                        `).join("")}
                      </div>
                    </div>
                  `}return`
                  <div class="state-entry">
                    <span class="state-key">${a}</span>
                    <span class="state-value">${B(t,o)}</span>
                  </div>
                `}).join("")}
            </div>
          </dd>
        </div>
      `}return`
      <div class="detail-field">
        <dt>${q(t)}</dt>
        <dd>${B(t,n)}</dd>
      </div>
    `}).join("")}function q(e){return e.replace(/_/g," ").replace(/\b\w/g,t=>t.toUpperCase())}function B(e,t){return $e(e),`<strong>${t}</strong>`}function Te(){if(typeof wallet<"u"&&wallet.getAccount){const e=wallet.getAccount();return(e==null?void 0:e.address)||null}return null}function Le(e,t){return e?e==="wallet"?Te()||"":e==="user"?(u==null?void 0:u.id)||(u==null?void 0:u.login)||"":(e.startsWith("balances.")||e.includes("."),""):""}function Ce(){return`
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
  `}let _=null;function Re(e){const t=F.find(s=>s.id===e);if(!t)return;_=e,document.getElementById("action-modal-title").textContent=t.name;const n=t.fields.map(s=>{const o=Le(s.autoFill,i==null?void 0:i.state)||s.defaultValue||"",c=s.required?"required":"";let r="";if(s.type==="amount")r=`
        <input
          type="number"
          name="${s.name}"
          value="${o}"
          placeholder="${s.placeholder||"Amount"}"
          step="any"
          ${c}
          class="form-control"
        />
        
      `;else if(s.type==="address"){const l=je();l.length>0?r=`
          <select name="${s.name}" ${c} class="form-control">
            <option value="">Select address...</option>
            ${l.map(d=>`
              <option value="${d.address}" ${d.address===o?"selected":""}>
                ${d.name||"Account"} (${d.address.slice(0,8)}...${d.address.slice(-6)})
              </option>
            `).join("")}
          </select>
        `:r=`
          <input
            type="text"
            name="${s.name}"
            value="${o}"
            placeholder="${s.placeholder||"0x..."}"
            ${c}
            class="form-control"
          />
        `}else s.type==="hidden"?r=`<input type="hidden" name="${s.name}" value="${o}" />`:r=`
        <input
          type="${s.type==="number"?"number":"text"}"
          name="${s.name}"
          value="${o}"
          placeholder="${s.placeholder||""}"
          ${c}
          class="form-control"
        />
      `;return s.type==="hidden"?r:`
      <div class="form-field">
        <label>${s.label}${s.required?" *":""}</label>
        ${r}
      </div>
    `}).join("");document.getElementById("action-form-fields").innerHTML=n,document.getElementById("action-modal").style.display="flex"}window.hideActionModal=function(){document.getElementById("action-modal").style.display="none",_=null};window.handleActionSubmit=async function(e){var c;if(e.preventDefault(),!_||!i)return;const t=_,n=i.id,s=e.target,a=new FormData(s),o={};for(const[r,l]of a.entries()){const d=(c=F.find(T=>T.id===t))==null?void 0:c.fields.find(T=>T.name===r);d&&(d.type==="amount"||d.type==="number")?o[r]=parseFloat(l)||0:o[r]=l}hideActionModal();try{const r=await g.executeTransition(t,n,o);i={...i,version:r.version,state:r.state,displayState:r.state,places:r.state,enabled:r.enabled||[]},window.currentInstanceState=i.state,C(),W(`Action "${t}" completed!`)}catch(r){A(`Failed to execute ${t}: ${r.message}`)}};function je(){return typeof wallet<"u"&&wallet.getAccounts?wallet.getAccounts()||[]:[]}window.showAddressPicker=function(e){if(typeof wallet>"u"||!wallet.getAccounts)return;const t=wallet.getAccounts();if(!t||t.length===0)return;const n=document.querySelector(".address-picker-dropdown");n&&n.remove();const s=document.querySelector(`[name="${e}"]`);if(!s)return;const a=s.getBoundingClientRect(),o=document.createElement("div");o.className="address-picker-dropdown",o.style.cssText=`
    position: fixed;
    top: ${a.bottom+4}px;
    left: ${a.left}px;
    width: ${a.width}px;
    background: white;
    border: 1px solid #ddd;
    border-radius: 4px;
    box-shadow: 0 4px 12px rgba(0,0,0,0.15);
    z-index: 2000;
    max-height: 200px;
    overflow-y: auto;
  `,o.innerHTML=t.map(c=>`
    <div class="address-picker-option" onclick="selectAddress('${e}', '${c.address}')" style="
      padding: 8px 12px;
      cursor: pointer;
      border-bottom: 1px solid #eee;
    ">
      <div style="font-weight: 500;">${c.name||"Account"}</div>
      <div style="font-size: 0.85rem; color: #666; font-family: monospace;">${c.address.slice(0,10)}...${c.address.slice(-8)}</div>
    </div>
  `).join(""),document.body.appendChild(o),setTimeout(()=>{document.addEventListener("click",function c(r){o.contains(r.target)||(o.remove(),document.removeEventListener("click",c))})},0)};window.selectAddress=function(e,t){const n=document.querySelector(`[name="${e}"]`);n&&(n.value=t,n.dispatchEvent(new Event("input",{bubbles:!0})));const s=document.querySelector(".address-picker-dropdown");s&&s.remove()};async function Ne(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/test-access')" style="margin-left: -0.5rem">
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
            <button type="button" class="btn btn-secondary" onclick="navigate('/test-access')">Cancel</button>
          </div>
        </form>
      </div>
    </div>
  `}async function Ie(){var t,n,s,a,o,c,r;const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>Schema Viewer</h1>
        <p style="color: #666; margin-top: 0.5rem;">Inspect the Petri net model that powers this application</p>
      </div>
      <div id="schema-content" class="card">
        <div class="loading">Loading schema...</div>
      </div>
    </div>
  `;try{const d=await(await fetch(`${b}/api/schema`)).json(),T=document.getElementById("schema-content");T.innerHTML=`
      <div class="schema-viewer">
        <div class="schema-tabs">
          <button class="schema-tab active" onclick="showSchemaTab('overview')">Overview</button>
          <button class="schema-tab" onclick="showSchemaTab('places')">Places (${((t=d.places)==null?void 0:t.length)||0})</button>
          <button class="schema-tab" onclick="showSchemaTab('transitions')">Transitions (${((n=d.transitions)==null?void 0:n.length)||0})</button>
          <button class="schema-tab" onclick="showSchemaTab('arcs')">Arcs (${((s=d.arcs)==null?void 0:s.length)||0})</button>
          <button class="schema-tab" onclick="showSchemaTab('raw')">Raw JSON</button>
        </div>

        <div id="schema-tab-overview" class="schema-tab-content active">
          <div class="schema-overview">
            <div class="schema-info-card">
              <h3>${d.name||"Unnamed"}</h3>
              <p>${d.description||"No description"}</p>
            </div>
            <div class="schema-stats">
              <div class="stat-item">
                <span class="stat-value">${((a=d.places)==null?void 0:a.length)||0}</span>
                <span class="stat-label">Places</span>
              </div>
              <div class="stat-item">
                <span class="stat-value">${((o=d.transitions)==null?void 0:o.length)||0}</span>
                <span class="stat-label">Transitions</span>
              </div>
              <div class="stat-item">
                <span class="stat-value">${((c=d.arcs)==null?void 0:c.length)||0}</span>
                <span class="stat-label">Arcs</span>
              </div>
              <div class="stat-item">
                <span class="stat-value">${((r=d.roles)==null?void 0:r.length)||0}</span>
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
              ${(d.places||[]).map(m=>`
                <tr>
                  <td><code>${m.id}</code></td>
                  <td>${m.description||"-"}</td>
                  <td>${m.initial||0}</td>
                  <td>${m.capacity||"∞"}</td>
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
              ${(d.transitions||[]).map(m=>`
                <tr>
                  <td><code>${m.id}</code></td>
                  <td>${m.description||"-"}</td>
                  <td><code>${m.guard||"-"}</code></td>
                  <td>${(m.roles||[]).join(", ")||"-"}</td>
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
              ${(d.arcs||[]).map(m=>`
                <tr>
                  <td><code>${m.from}</code></td>
                  <td><code>${m.to}</code></td>
                  <td>${m.weight||1}</td>
                  <td>${m.type||"normal"}</td>
                </tr>
              `).join("")}
            </tbody>
          </table>
        </div>

        <div id="schema-tab-raw" class="schema-tab-content" style="display: none;">
          <pre class="schema-json">${JSON.stringify(d,null,2)}</pre>
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
      </style>
    `}catch(l){console.error("Failed to load schema:",l),document.getElementById("schema-content").innerHTML=`
      <div class="error">Failed to load schema: ${l.message}</div>
    `}}window.showSchemaTab=function(e){document.querySelectorAll(".schema-tab").forEach(n=>{n.classList.remove("active")}),event.target.classList.add("active"),document.querySelectorAll(".schema-tab-content").forEach(n=>{n.style.display="none",n.classList.remove("active")});const t=document.getElementById(`schema-tab-${e}`);t&&(t.style.display="block",t.classList.add("active"))};async function _e(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>Admin Dashboard</h1>
      </div>
      <div id="admin-stats" class="card">
        <div class="loading">Loading statistics...</div>
      </div>
      <div id="admin-instances" class="card">
        <div class="card-header">Recent Instances</div>
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{const[t,n]=await Promise.all([fetch(`${b}/admin/stats`,{headers:v()}).then(a=>a.json()).catch(()=>null),g.listInstances()]);t?document.getElementById("admin-stats").innerHTML=`
        <div class="card-header">Statistics</div>
        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 1rem;">
          <div>
            <div style="font-size: 2rem; font-weight: 600;">${t.total_streams||0}</div>
            <div style="color: #666;">Total Instances</div>
          </div>
          <div>
            <div style="font-size: 2rem; font-weight: 600;">${t.total_events||0}</div>
            <div style="color: #666;">Total Events</div>
          </div>
        </div>
      `:document.getElementById("admin-stats").innerHTML="",y=n.instances||[];const s=document.getElementById("admin-instances").querySelector(".loading");s&&(s.outerHTML=y.length>0?`<table class="table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Status</th>
                <th>Version</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              ${y.slice(0,20).map(a=>{const o=U(a.state||a.places);return`
                  <tr>
                    <td><code>${a.id}</code></td>
                    <td>${V(o)}</td>
                    <td>${a.version||0}</td>
                    <td><button class="btn btn-sm btn-link" onclick="navigate('/test-access/${a.id}')">View</button></td>
                  </tr>
                `}).join("")}
            </tbody>
          </table>`:'<p style="color: #666; padding: 1rem;">No instances yet.</p>')}catch(t){A("Failed to load admin data: "+t.message)}}window.navigate=h;window.handleCreateNew=async function(){h("/test-access/new")};window.handleSubmitCreate=async function(e){e.preventDefault();try{const t=await g.createInstance({});W("Instance created successfully!"),h(`/test-access/${t.aggregate_id||t.id}`)}catch(t){A("Failed to create: "+t.message)}};window.handleTransition=async function(e){if(!i)return;const t=F.find(n=>n.id===e);if(t&&t.fields&&t.fields.length>0){Re(e);return}try{const n=await g.executeTransition(e,i.id);i={...i,version:n.version,state:n.state,displayState:n.state,places:n.state,enabled:n.enabled||[]},window.currentInstanceState=i.state,C(),W(`Action "${e}" completed!`)}catch(n){A(`Failed to execute ${e}: ${n.message}`)}};function Y(e){var s;const t=((s=e.detail)==null?void 0:s.route)||L();if(!t){M();return}const n=t.path;n==="/test-access"||n==="/"?M():n==="/test-access/new"?Ne():n==="/test-access/:id"?Se():n==="/schema"?Ie():n==="/admin"||n.startsWith("/admin")?_e():M()}async function Oe(){const e=new URLSearchParams(window.location.search),t=e.get("token"),n=e.get("expires_at");if(t){f=t;try{const s=await g.getMe();J({token:t,expires_at:n,user:s}),window.history.replaceState({},"",window.location.pathname),await O()}catch{R(),A("Failed to complete login")}}}async function Fe(){xe(),await Oe(),await we();const e=document.getElementById("nav");e.innerHTML=await te();const t=document.createElement("div");t.innerHTML=Ce(),document.body.appendChild(t),window.addEventListener("route-change",Y),be(),Y({detail:{route:L()}})}let w=null,j=null;function se(){const t=`${window.location.protocol==="https:"?"wss:":"ws:"}//${window.location.host}/ws`;w=new WebSocket(t),w.onopen=()=>{console.log("[Debug] WebSocket connected")},w.onmessage=n=>{try{const s=JSON.parse(n.data);s.id==="session"&&s.type==="session"?(j=(typeof s.data=="string"?JSON.parse(s.data):s.data).session_id,console.log("[Debug] Session ID:",j)):s.type==="eval"&&Pe(s)}catch(s){console.error("[Debug] Failed to parse message:",s)}},w.onclose=()=>{console.log("[Debug] WebSocket disconnected, reconnecting in 3s..."),j=null,setTimeout(se,3e3)},w.onerror=n=>{console.error("[Debug] WebSocket error:",n)}}async function Pe(e){try{const n=(typeof e.data=="string"?JSON.parse(e.data):e.data).code,a=await new Function("return (async () => { "+n+" })()")(),o={type:"response",id:e.id,data:{result:a,type:typeof a}};w.send(JSON.stringify(o))}catch(t){const n={type:"response",id:e.id,data:{error:t.message}};w.send(JSON.stringify(n))}}window.debugSessionId=()=>j;window.debugWs=()=>w;window.pilot={async list(){return h("/test-access"),await this.waitFor(".entity-card, .empty-state",5e3).catch(()=>{}),y},newForm(){return h("/test-access/new"),this.waitForRender()},async view(e){return h(`/test-access/${e}`),await this.waitForRender(),i},admin(){return h("/admin"),this.waitForRender()},async create(e={}){const t=await g.createInstance(e),n=t.aggregate_id||t.id;return h(`/test-access/${n}`),await this.waitForRender(),{id:n,...t}},getCurrentInstance(){return i},getInstances(){return y},async refresh(){if(!i)throw new Error("No current instance");const e=await g.getInstance(i.id);return i={id:e.aggregate_id||i.id,version:e.version,state:e.state,places:e.places,enabled:e.enabled||e.enabled_transitions||[]},C(),i},async action(e,t={}){if(!i)throw new Error("No current instance - navigate to detail page first");const n=await g.executeTransition(e,i.id,t);return i={...i,version:n.version,state:n.state,places:n.state,enabled:n.enabled||[]},C(),{success:!0,state:i.places,enabled:i.enabled}},isEnabled(e){return i?(i.enabled||[]).includes(e):!1},getEnabled(){return(i==null?void 0:i.enabled)||[]},fill(e,t){const n=document.querySelector(`[name="${e}"]`);if(!n)throw new Error(`No input found with name: ${e}`);return n.value=t,n.dispatchEvent(new Event("input",{bubbles:!0})),this},async submit(){const e=document.querySelector("form");if(!e)throw new Error("No form found on page");const t=new Event("submit",{bubbles:!0,cancelable:!0});return e.dispatchEvent(t),await this.waitForRender(),i},getText(e){const t=document.querySelector(e);return t?t.textContent.trim():null},exists(e){return document.querySelector(e)!==null},getButtons(){return Array.from(document.querySelectorAll("button")).map(e=>({text:e.textContent.trim(),disabled:e.disabled,className:e.className}))},async clickButton(e){const t=document.querySelectorAll("button");for(const n of t)if(n.textContent.trim()===e&&!n.disabled)return n.click(),await this.waitForRender(),!0;throw new Error(`No enabled button found with text: ${e}`)},getState(){return(i==null?void 0:i.places)||null},getStatus(){if(!(i!=null&&i.places))return null;for(const[e,t]of Object.entries(i.places))if(t>0)return e;return null},getRoute(){return L()},getUser(){return u},isAuthenticated(){return f!==null},waitForRender(e=50){return new Promise(t=>setTimeout(t,e))},async waitFor(e,t=5e3){const n=Date.now();for(;Date.now()-n<t;){if(document.querySelector(e))return document.querySelector(e);await this.waitForRender(50)}throw new Error(`Timeout waiting for: ${e}`)},async waitForState(e,t=5e3){var s;const n=Date.now();for(;Date.now()-n<t;){if(((s=i==null?void 0:i.places)==null?void 0:s[e])>0)return i;await this.waitForRender(100)}throw new Error(`Timeout waiting for state: ${e}`)},debug(){return console.log("=== Pilot Debug ==="),console.log("Route:",L()),console.log("User:",u),console.log("Instance:",i),console.log("Enabled:",i==null?void 0:i.enabled),console.log("State:",i==null?void 0:i.places),{route:L(),user:u,instance:i}},async getEvents(){if(!i)throw new Error("No current instance");const e=await fetch(`${b}/api/testaccess/${i.id}/events`,{headers:v()});return(await S(e)).events||[]},async getEventCount(){return(await this.getEvents()).length},async getLastEvent(){const e=await this.getEvents();return e.length>0?e[e.length-1]:null},async replayTo(e){if(!i)throw new Error("No current instance");const n=(await this.getEvents()).filter(a=>(a.version||a.sequence)<=e),s={};for(const a of n)a.state&&Object.assign(s,a.state);return{version:e,events:n,places:s}},async loginAs(e){const t=typeof e=="string"?[e]:e,s=await(await fetch(`${b}/api/debug/login`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({login:"pilot-user",roles:t})})).json();return J(s),await this.waitForRender(100),s},logout(){return R(),this.waitForRender()},getRoles(){return(u==null?void 0:u.roles)||[]},hasRole(e){return this.getRoles().includes(e)},assertState(e){const t=this.getStatus();if(t!==e)throw new Error(`Expected state '${e}', got '${t}'`);return this},assertEnabled(e){if(!this.isEnabled(e)){const t=this.getEnabled();throw new Error(`Expected '${e}' to be enabled. Enabled: [${t.join(", ")}]`)}return this},assertDisabled(e){if(this.isEnabled(e))throw new Error(`Expected '${e}' to be disabled, but it is enabled`);return this},assertExists(e){if(!this.exists(e))throw new Error(`Expected element '${e}' to exist`);return this},assertText(e,t){const n=this.getText(e);if(n!==t)throw new Error(`Expected '${e}' to contain '${t}', got '${n}'`);return this},assertAuthenticated(){if(!this.isAuthenticated())throw new Error("Expected user to be authenticated");return this},assertRole(e){if(!this.hasRole(e))throw new Error(`Expected user to have role '${e}'. Has: [${this.getRoles().join(", ")}]`);return this},getTransitions(){return[{id:"submit",name:"Submit",description:"Submit for review",requiredRoles:["customer"],apiPath:"/items/{id}/submit"},{id:"approve",name:"Approve",description:"Approve the submission",requiredRoles:["reviewer"],apiPath:"/items/{id}/approve"},{id:"reject",name:"Reject",description:"Reject the submission",requiredRoles:["reviewer"],apiPath:"/items/{id}/reject"}]},getPlaces(){return[{id:"draft",name:"Draft",initial:1},{id:"submitted",name:"Submitted",initial:0},{id:"approved",name:"Approved",initial:0},{id:"rejected",name:"Rejected",initial:0}]},getTransition(e){return this.getTransitions().find(t=>t.id===e)||null},canFire(e){const t=this.getTransition(e);if(!t)return{canFire:!1,reason:`Unknown transition: ${e}`};if(!i)return{canFire:!1,reason:"No current instance"};if(!this.isEnabled(e)){const s=this.getStatus();return{canFire:!1,reason:`Transition '${e}' not enabled in state '${s}'`,currentState:s,enabledTransitions:this.getEnabled()}}if(t.requiredRoles&&t.requiredRoles.length>0){const s=this.getRoles();if(!t.requiredRoles.some(o=>s.includes(o)))return{canFire:!1,reason:`User lacks required role. Need one of: [${t.requiredRoles.join(", ")}]. Has: [${s.join(", ")}]`,requiredRoles:t.requiredRoles,userRoles:s}}return{canFire:!0}},async sequence(e,t={}){const n=[],{stopOnError:s=!0,data:a={}}=t;for(const o of e){const c=this.canFire(o);if(!c.canFire){if(s)throw new Error(`Sequence failed at '${o}': ${c.reason}`);n.push({transition:o,success:!1,error:c.reason});continue}try{const r=await this.action(o,a[o]||{});n.push({transition:o,success:!0,state:r.state})}catch(r){if(s)throw r;n.push({transition:o,success:!1,error:r.message})}}return n},getWorkflowInfo(){var e;return{places:this.getPlaces(),transitions:this.getTransitions(),initialPlace:(e=this.getPlaces().find(t=>t.initial>0))==null?void 0:e.id}}};Fe();se();
