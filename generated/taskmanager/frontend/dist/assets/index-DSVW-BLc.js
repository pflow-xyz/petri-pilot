(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const i of document.querySelectorAll('link[rel="modulepreload"]'))a(i);new MutationObserver(i=>{for(const o of i)if(o.type==="childList")for(const d of o.addedNodes)d.tagName==="LINK"&&d.rel==="modulepreload"&&a(d)}).observe(document,{childList:!0,subtree:!0});function n(i){const o={};return i.integrity&&(o.integrity=i.integrity),i.referrerPolicy&&(o.referrerPolicy=i.referrerPolicy),i.crossOrigin==="use-credentials"?o.credentials="include":i.crossOrigin==="anonymous"?o.credentials="omit":o.credentials="same-origin",o}function a(i){if(i.ep)return;i.ep=!0;const o=n(i);fetch(i.href,o)}})();const P=[{path:"/",component:"List",title:"task-manager"},{path:"/taskmanager",component:"List",title:"task-manager"},{path:"/taskmanager/new",component:"Form",title:"New task-manager"},{path:"/taskmanager/:id",component:"Detail",title:"task-manager Detail"},{path:"/admin",component:"AdminDashboard",title:"Admin Dashboard"},{path:"/admin/instances",component:"AdminInstances",title:"Instances"},{path:"/admin/instances/:id",component:"AdminInstance",title:"Instance Detail"}];let y=null,$={};function N(e){e=e||"/",e!=="/"&&e.endsWith("/")&&(e=e.slice(0,-1));for(const t of P){const n={};let a=t.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");a=a.replace(/:[^/]+/g,"([^/]+)");const i=new RegExp(`^${a}$`),o=e.match(i);if(o)return(t.path.match(/:[^/]+/g)||[]).map(r=>r.slice(1)).forEach((r,h)=>{n[r]=decodeURIComponent(o[h+1])}),{route:t,params:n}}return null}function u(e,t={}){e.startsWith("/")||(e="/"+e);const n=N(e);if(!n){console.warn(`No route found for path: ${e}, falling back to list`),e="/taskmanager";const a=N(e);a&&(y=a.route,$=a.params,window.history.pushState(t,"",e),B());return}if(n.route.roles&&n.route.roles.length>0){const a=Q();if(!a||!X(a,n.route.roles)){console.warn("Access denied:",e),u("/taskmanager");return}}y=n.route,$=n.params,window.history.pushState(t,"",e),B()}window.addEventListener("popstate",()=>{const e=window.location.pathname,t=N(e);t?(y=t.route,$=t.params,B()):u("/taskmanager")});function Q(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function X(e,t){return!e||!e.roles?!1:t.some(n=>e.roles.includes(n))}function B(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:y,params:$}}))}function Z(){return $}function S(){return y}function ee(){const e=window.location.pathname,t=N(e);t?(y=t.route,$=t.params):(y=P.find(n=>n.path==="/taskmanager")||P[0],$={})}const J=[{id:"user",description:""},{id:"reviewer",description:""},{id:"admin",description:""}],T={brand:"task-manager",items:[{label:"task-manager",path:"/taskmanager",icon:""},{label:"New",path:"/taskmanager/new",icon:"+"},{label:"Admin",path:"/admin",icon:""}]};let l=null,F=!1;async function U(){if(!F){F=!0;try{const e={},t=G();t&&(e.Authorization=`Bearer ${t}`);const n=await fetch("/api/navigation",{headers:e});n.ok?l=await n.json():l=T}catch{l=T}finally{F=!1}}}async function K(){l||await U();const e=window.location.pathname,t=ne(),n=(l==null?void 0:l.items)||T.items;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="/taskmanager" onclick="handleNavClick(event, '/taskmanager')">
          ${(l==null?void 0:l.brand)||T.brand}
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
    ${te()}
  `}function te(){return J.length===0?`
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
            ${J.map(t=>`
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
  `}window.showLoginModal=function(){const e=document.getElementById("login-modal");e&&(e.style.display="flex")};window.hideLoginModal=function(){const e=document.getElementById("login-modal");e&&(e.style.display="none")};window.handleRoleLogin=async function(e){try{const t=await fetch("/api/debug/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({login:"pilot-user",roles:e})});if(!t.ok)throw new Error("Login failed");const n=await t.json();localStorage.setItem("auth",JSON.stringify(n)),hideLoginModal(),l=null,window.dispatchEvent(new CustomEvent("auth-change")),await I()}catch(t){console.error("Login error:",t),alert("Login failed. Please try again.")}};window.handleNavClick=function(e,t){e.preventDefault(),u(t)};window.handleLogout=async function(){try{const e=G();e&&await fetch("/auth/logout",{method:"POST",headers:{Authorization:`Bearer ${e}`}})}catch(e){console.error("Logout error:",e)}localStorage.removeItem("auth"),l=null,window.dispatchEvent(new CustomEvent("auth-change")),await I(),u("/taskmanager")};function ne(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function G(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).token}catch{return null}return null}async function I(){l=null,await U();const e=document.getElementById("nav");e&&(e.innerHTML=await K())}window.addEventListener("auth-change",async()=>{await I()});window.addEventListener("route-change",()=>{const e=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(t=>{t.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(t=>{const n=t.getAttribute("href");(n===e||n!=="/"&&e.startsWith(n))&&t.parentElement.classList.add("active")})});let W=[];async function ae(){try{const e=await fetch("/api/views");return e.ok?(W=await e.json(),W):(console.warn("Failed to load view definitions, using defaults"),[])}catch(e){return console.error("Error loading views:",e),[]}}const f="",se="";function oe(e){return["amount","value","balance","total_supply","allowance"].some(n=>e.toLowerCase().includes(n))}let c=null,p=null,b=[],s=null;const R=[{id:"start",name:"Start",description:"Start working on a task",fields:[]},{id:"submit",name:"Submit",description:"Submit task for review",fields:[]},{id:"approve",name:"Approve",description:"Approve completed task",fields:[]},{id:"reject",name:"Reject",description:"Reject task and send back",fields:[]}];function ie(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);if(t.expires_at&&new Date(t.expires_at)>new Date)return p=t.token,c=t.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function q(e){localStorage.setItem("auth",JSON.stringify(e)),p=e.token,c=e.user,window.dispatchEvent(new CustomEvent("auth-change"))}function A(){localStorage.removeItem("auth"),p=null,c=null,window.dispatchEvent(new CustomEvent("auth-change"))}function re(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);return p=t.token,c=t.user,!0}catch{return!1}return p=null,c=null,!1}window.addEventListener("auth-change",()=>{re()});function w(){const e={"Content-Type":"application/json"};return p&&(e.Authorization=`Bearer ${p}`),e}async function k(e){if(e.status===401)throw A(),E("Session expired. Please log in again."),new Error("Unauthorized");if(!e.ok){const t=await e.json().catch(()=>({}));throw new Error(t.message||e.statusText)}return e.json()}const m={async getMe(){const e=await fetch(`${f}/auth/me`,{headers:w()});return k(e)},async logout(){await fetch(`${f}/auth/logout`,{method:"POST",headers:w()}),A()},async listInstances(){const e=await fetch(`${f}/admin/instances`,{headers:w()});return k(e)},async getInstance(e){const t=await fetch(`${f}/api/taskmanager/${e}`,{headers:w()});return k(t)},async createInstance(e={}){const t=await fetch(`${f}/api/taskmanager`,{method:"POST",headers:w(),body:JSON.stringify(e)});return k(t)},async executeTransition(e,t,n={}){const a=n,i=await fetch(`${f}/api/${e}`,{method:"POST",headers:w(),body:JSON.stringify({aggregate_id:t,data:a})});return k(i)}};window.api=m;Object.defineProperty(window,"currentInstance",{get:function(){return s}});window.setAuthToken=function(e){p=e};window.saveAuth=q;window.clearAuth=A;function E(e){const t=document.getElementById("app"),n=t.querySelector(".alert-error");n&&n.remove();const a=document.createElement("div");a.className="alert alert-error",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),5e3)}function _(e){const t=document.getElementById("app"),n=t.querySelector(".alert-success");n&&n.remove();const a=document.createElement("div");a.className="alert alert-success",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),3e3)}function H(e){if(!e)return"unknown";for(const[t,n]of Object.entries(e))if(n>0)return t;return"unknown"}function z(e){return`<span class="badge ${`badge-${e.toLowerCase().replace(/_/g,"-")}`}">${e.replace(/_/g," ")}</span>`}async function O(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>task-manager</h1>
        <button class="btn btn-primary" onclick="handleCreateNew()">+ New</button>
      </div>
      <div id="instances-list" class="entity-list">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{b=(await m.listInstances()).instances||[],de()}catch{document.getElementById("instances-list").innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `}}function de(){const e=document.getElementById("instances-list");if(e){if(b.length===0){e.innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `;return}e.innerHTML=b.map(t=>{const n=H(t.state||t.places);return`
      <div class="entity-card" onclick="navigate('/taskmanager/${t.id}')">
        <div class="entity-info">
          <h3>${t.id}</h3>
          <div class="entity-meta">
            ${z(n)} &middot; Version ${t.version||0}
          </div>
        </div>
        <div class="entity-actions">
          <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); navigate('/taskmanager/${t.id}')">
            View
          </button>
        </div>
      </div>
    `}).join("")}}async function ce(){const t=Z().id,n=document.getElementById("app");n.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/taskmanager')" style="margin-left: -0.5rem">
            &larr; Back to List
          </button>
          <h1 style="margin-top: 0.5rem">Instance: ${t}</h1>
        </div>
      </div>
      <div id="instance-detail">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{const a=await m.getInstance(t);s={id:a.aggregate_id||t,version:a.version,state:a.state,displayState:a.state,places:a.places,enabled:a.enabled||a.enabled_transitions||[]},window.currentInstanceState=s.state,x()}catch(a){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${a.message}</div>
    `}}function x(){const e=document.getElementById("instance-detail");if(!e||!s)return;const t=H(s.places),n=s.enabled||[],a=R;e.innerHTML=`
    <div class="card">
      <div class="card-header">Status</div>
      <div class="detail-list">
        <div class="detail-field">
          <dt>ID</dt>
          <dd><code>${s.id}</code></dd>
        </div>
        <div class="detail-field">
          <dt>Status</dt>
          <dd>${z(t)}</dd>
        </div>
        <div class="detail-field">
          <dt>Version</dt>
          <dd>${s.version||0}</dd>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="card-header">Actions</div>
      <div class="view-actions">
        ${a.map(i=>{const o=n.includes(i.id);return`
            <button
              class="btn ${o?"btn-primary":"btn-secondary"}"
              onclick="handleTransition('${i.id}')"
              ${o?"":"disabled"}
              title="${i.description||i.name}"
            >
              ${i.name}
            </button>
          `}).join("")}
      </div>
      ${n.length===0?'<p style="color: #666; margin-top: 1rem;">No actions available in current state.</p>':""}
    </div>

    <div class="card">
      <div class="card-header">Current State</div>
      <div class="detail-list">
        ${le(s.displayState||s.state)}
      </div>
    </div>
  `}function le(e){return!e||Object.keys(e).length===0?'<p style="color: #999;">No state data</p>':Object.entries(e).map(([t,n])=>{if(typeof n=="object"&&n!==null){const a=Object.entries(n);return a.length===0?`
          <div class="detail-field">
            <dt>${M(t)}</dt>
            <dd><span style="color: #999;">Empty</span></dd>
          </div>
        `:`
        <div class="detail-field">
          <dt>${M(t)}</dt>
          <dd>
            <div class="nested-state">
              ${a.map(([i,o])=>{if(typeof o=="object"&&o!==null){const d=Object.entries(o);return d.length===0?`
                      <div class="state-entry">
                        <span class="state-key">${i}</span>
                        <span class="state-value" style="color: #999;">Empty</span>
                      </div>
                    `:`
                    <div class="state-entry nested-group">
                      <span class="state-key">${i}</span>
                      <div class="nested-state" style="margin-left: 1rem;">
                        ${d.map(([r,h])=>`
                          <div class="state-entry">
                            <span class="state-key">${r}</span>
                            <span class="state-value">${D(t,h)}</span>
                          </div>
                        `).join("")}
                      </div>
                    </div>
                  `}return`
                  <div class="state-entry">
                    <span class="state-key">${i}</span>
                    <span class="state-value">${D(t,o)}</span>
                  </div>
                `}).join("")}
            </div>
          </dd>
        </div>
      `}return`
      <div class="detail-field">
        <dt>${M(t)}</dt>
        <dd>${D(t,n)}</dd>
      </div>
    `}).join("")}function M(e){return e.replace(/_/g," ").replace(/\b\w/g,t=>t.toUpperCase())}function D(e,t){return oe(e),`<strong>${t}</strong>`}function ue(){if(typeof wallet<"u"&&wallet.getAccount){const e=wallet.getAccount();return(e==null?void 0:e.address)||null}return null}function me(e,t){return e?e==="wallet"?ue()||"":e==="user"?(c==null?void 0:c.id)||(c==null?void 0:c.login)||"":(e.startsWith("balances.")||e.includes("."),""):""}function pe(){return`
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
  `}let C=null;function he(e){const t=R.find(a=>a.id===e);if(!t)return;C=e,document.getElementById("action-modal-title").textContent=t.name;const n=t.fields.map(a=>{const o=me(a.autoFill,s==null?void 0:s.state)||a.defaultValue||"",d=a.required?"required":"";let r="";if(a.type==="amount")r=`
        <input
          type="number"
          name="${a.name}"
          value="${o}"
          placeholder="${a.placeholder||"Amount"}"
          step="any"
          ${d}
          class="form-control"
        />
        
      `;else if(a.type==="address"){const h=ge();h.length>0?r=`
          <select name="${a.name}" ${d} class="form-control">
            <option value="">Select address...</option>
            ${h.map(g=>`
              <option value="${g.address}" ${g.address===o?"selected":""}>
                ${g.name||"Account"} (${g.address.slice(0,8)}...${g.address.slice(-6)})
              </option>
            `).join("")}
          </select>
        `:r=`
          <input
            type="text"
            name="${a.name}"
            value="${o}"
            placeholder="${a.placeholder||"0x..."}"
            ${d}
            class="form-control"
          />
        `}else a.type==="hidden"?r=`<input type="hidden" name="${a.name}" value="${o}" />`:r=`
        <input
          type="${a.type==="number"?"number":"text"}"
          name="${a.name}"
          value="${o}"
          placeholder="${a.placeholder||""}"
          ${d}
          class="form-control"
        />
      `;return a.type==="hidden"?r:`
      <div class="form-field">
        <label>${a.label}${a.required?" *":""}</label>
        ${r}
      </div>
    `}).join("");document.getElementById("action-form-fields").innerHTML=n,document.getElementById("action-modal").style.display="flex"}window.hideActionModal=function(){document.getElementById("action-modal").style.display="none",C=null};window.handleActionSubmit=async function(e){var d;if(e.preventDefault(),!C||!s)return;const t=C,n=s.id,a=e.target,i=new FormData(a),o={};for(const[r,h]of i.entries()){const g=(d=R.find(j=>j.id===t))==null?void 0:d.fields.find(j=>j.name===r);g&&(g.type==="amount"||g.type==="number")?o[r]=parseFloat(h)||0:o[r]=h}hideActionModal();try{const r=await m.executeTransition(t,n,o);s={...s,version:r.version,state:r.state,displayState:r.state,places:r.state,enabled:r.enabled||[]},window.currentInstanceState=s.state,x(),_(`Action "${t}" completed!`)}catch(r){E(`Failed to execute ${t}: ${r.message}`)}};function ge(){return typeof wallet<"u"&&wallet.getAccounts?wallet.getAccounts()||[]:[]}window.showAddressPicker=function(e){if(typeof wallet>"u"||!wallet.getAccounts)return;const t=wallet.getAccounts();if(!t||t.length===0)return;const n=document.querySelector(".address-picker-dropdown");n&&n.remove();const a=document.querySelector(`[name="${e}"]`);if(!a)return;const i=a.getBoundingClientRect(),o=document.createElement("div");o.className="address-picker-dropdown",o.style.cssText=`
    position: fixed;
    top: ${i.bottom+4}px;
    left: ${i.left}px;
    width: ${i.width}px;
    background: white;
    border: 1px solid #ddd;
    border-radius: 4px;
    box-shadow: 0 4px 12px rgba(0,0,0,0.15);
    z-index: 2000;
    max-height: 200px;
    overflow-y: auto;
  `,o.innerHTML=t.map(d=>`
    <div class="address-picker-option" onclick="selectAddress('${e}', '${d.address}')" style="
      padding: 8px 12px;
      cursor: pointer;
      border-bottom: 1px solid #eee;
    ">
      <div style="font-weight: 500;">${d.name||"Account"}</div>
      <div style="font-size: 0.85rem; color: #666; font-family: monospace;">${d.address.slice(0,10)}...${d.address.slice(-8)}</div>
    </div>
  `).join(""),document.body.appendChild(o),setTimeout(()=>{document.addEventListener("click",function d(r){o.contains(r.target)||(o.remove(),document.removeEventListener("click",d))})},0)};window.selectAddress=function(e,t){const n=document.querySelector(`[name="${e}"]`);n&&(n.value=t,n.dispatchEvent(new Event("input",{bubbles:!0})));const a=document.querySelector(".address-picker-dropdown");a&&a.remove()};async function fe(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/taskmanager')" style="margin-left: -0.5rem">
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
            <button type="button" class="btn btn-secondary" onclick="navigate('/taskmanager')">Cancel</button>
          </div>
        </form>
      </div>
    </div>
  `}async function we(){const e=document.getElementById("app");e.innerHTML=`
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
  `;try{const[t,n]=await Promise.all([fetch(`${f}/admin/stats`,{headers:w()}).then(i=>i.json()).catch(()=>null),m.listInstances()]);t?document.getElementById("admin-stats").innerHTML=`
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
      `:document.getElementById("admin-stats").innerHTML="",b=n.instances||[];const a=document.getElementById("admin-instances").querySelector(".loading");a&&(a.outerHTML=b.length>0?`<table class="table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Status</th>
                <th>Version</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              ${b.slice(0,20).map(i=>{const o=H(i.state||i.places);return`
                  <tr>
                    <td><code>${i.id}</code></td>
                    <td>${z(o)}</td>
                    <td>${i.version||0}</td>
                    <td><button class="btn btn-sm btn-link" onclick="navigate('/taskmanager/${i.id}')">View</button></td>
                  </tr>
                `}).join("")}
            </tbody>
          </table>`:'<p style="color: #666; padding: 1rem;">No instances yet.</p>')}catch(t){E("Failed to load admin data: "+t.message)}}window.navigate=u;window.handleCreateNew=async function(){u("/taskmanager/new")};window.handleSubmitCreate=async function(e){e.preventDefault();try{const t=await m.createInstance({});_("Instance created successfully!"),u(`/taskmanager/${t.aggregate_id||t.id}`)}catch(t){E("Failed to create: "+t.message)}};window.handleTransition=async function(e){if(!s)return;const t=R.find(n=>n.id===e);if(t&&t.fields&&t.fields.length>0){he(e);return}try{const n=await m.executeTransition(e,s.id);s={...s,version:n.version,state:n.state,displayState:n.state,places:n.state,enabled:n.enabled||[]},window.currentInstanceState=s.state,x(),_(`Action "${e}" completed!`)}catch(n){E(`Failed to execute ${e}: ${n.message}`)}};function V(e){var a;const t=((a=e.detail)==null?void 0:a.route)||S();if(!t){O();return}const n=t.path;n==="/taskmanager"||n==="/"?O():n==="/taskmanager/new"?fe():n==="/taskmanager/:id"?ce():n==="/admin"||n.startsWith("/admin")?we():O()}async function ve(){const e=new URLSearchParams(window.location.search),t=e.get("token"),n=e.get("expires_at");if(t){p=t;try{const a=await m.getMe();q({token:t,expires_at:n,user:a}),window.history.replaceState({},"",window.location.pathname),await I()}catch{A(),E("Failed to complete login")}}}async function be(){ie(),await ve(),await ae();const e=document.getElementById("nav");e.innerHTML=await K();const t=document.createElement("div");t.innerHTML=pe(),document.body.appendChild(t),window.addEventListener("route-change",V),ee(),V({detail:{route:S()}})}let v=null,L=null;function Y(){const t=`${window.location.protocol==="https:"?"wss:":"ws:"}//${window.location.host}/ws`;v=new WebSocket(t),v.onopen=()=>{console.log("[Debug] WebSocket connected")},v.onmessage=n=>{try{const a=JSON.parse(n.data);a.id==="session"&&a.type==="session"?(L=(typeof a.data=="string"?JSON.parse(a.data):a.data).session_id,console.log("[Debug] Session ID:",L)):a.type==="eval"&&ye(a)}catch(a){console.error("[Debug] Failed to parse message:",a)}},v.onclose=()=>{console.log("[Debug] WebSocket disconnected, reconnecting in 3s..."),L=null,setTimeout(Y,3e3)},v.onerror=n=>{console.error("[Debug] WebSocket error:",n)}}async function ye(e){try{const n=(typeof e.data=="string"?JSON.parse(e.data):e.data).code,i=await new Function("return (async () => { "+n+" })()")(),o={type:"response",id:e.id,data:{result:i,type:typeof i}};v.send(JSON.stringify(o))}catch(t){const n={type:"response",id:e.id,data:{error:t.message}};v.send(JSON.stringify(n))}}window.debugSessionId=()=>L;window.debugWs=()=>v;window.pilot={async list(){return u("/taskmanager"),await this.waitFor(".entity-card, .empty-state",5e3).catch(()=>{}),b},newForm(){return u("/taskmanager/new"),this.waitForRender()},async view(e){return u(`/taskmanager/${e}`),await this.waitForRender(),s},admin(){return u("/admin"),this.waitForRender()},async create(e={}){const t=await m.createInstance(e),n=t.aggregate_id||t.id;return u(`/taskmanager/${n}`),await this.waitForRender(),{id:n,...t}},getCurrentInstance(){return s},getInstances(){return b},async refresh(){if(!s)throw new Error("No current instance");const e=await m.getInstance(s.id);return s={id:e.aggregate_id||s.id,version:e.version,state:e.state,places:e.places,enabled:e.enabled||e.enabled_transitions||[]},x(),s},async action(e,t={}){if(!s)throw new Error("No current instance - navigate to detail page first");const n=await m.executeTransition(e,s.id,t);return s={...s,version:n.version,state:n.state,places:n.state,enabled:n.enabled||[]},x(),{success:!0,state:s.places,enabled:s.enabled}},isEnabled(e){return s?(s.enabled||[]).includes(e):!1},getEnabled(){return(s==null?void 0:s.enabled)||[]},fill(e,t){const n=document.querySelector(`[name="${e}"]`);if(!n)throw new Error(`No input found with name: ${e}`);return n.value=t,n.dispatchEvent(new Event("input",{bubbles:!0})),this},async submit(){const e=document.querySelector("form");if(!e)throw new Error("No form found on page");const t=new Event("submit",{bubbles:!0,cancelable:!0});return e.dispatchEvent(t),await this.waitForRender(),s},getText(e){const t=document.querySelector(e);return t?t.textContent.trim():null},exists(e){return document.querySelector(e)!==null},getButtons(){return Array.from(document.querySelectorAll("button")).map(e=>({text:e.textContent.trim(),disabled:e.disabled,className:e.className}))},async clickButton(e){const t=document.querySelectorAll("button");for(const n of t)if(n.textContent.trim()===e&&!n.disabled)return n.click(),await this.waitForRender(),!0;throw new Error(`No enabled button found with text: ${e}`)},getState(){return(s==null?void 0:s.places)||null},getStatus(){if(!(s!=null&&s.places))return null;for(const[e,t]of Object.entries(s.places))if(t>0)return e;return null},getRoute(){return S()},getUser(){return c},isAuthenticated(){return p!==null},waitForRender(e=50){return new Promise(t=>setTimeout(t,e))},async waitFor(e,t=5e3){const n=Date.now();for(;Date.now()-n<t;){if(document.querySelector(e))return document.querySelector(e);await this.waitForRender(50)}throw new Error(`Timeout waiting for: ${e}`)},async waitForState(e,t=5e3){var a;const n=Date.now();for(;Date.now()-n<t;){if(((a=s==null?void 0:s.places)==null?void 0:a[e])>0)return s;await this.waitForRender(100)}throw new Error(`Timeout waiting for state: ${e}`)},debug(){return console.log("=== Pilot Debug ==="),console.log("Route:",S()),console.log("User:",c),console.log("Instance:",s),console.log("Enabled:",s==null?void 0:s.enabled),console.log("State:",s==null?void 0:s.places),{route:S(),user:c,instance:s}},async getEvents(){if(!s)throw new Error("No current instance");const e=await fetch(`${f}/api/taskmanager/${s.id}/events`,{headers:w()});return(await k(e)).events||[]},async getEventCount(){return(await this.getEvents()).length},async getLastEvent(){const e=await this.getEvents();return e.length>0?e[e.length-1]:null},async replayTo(e){if(!s)throw new Error("No current instance");const n=(await this.getEvents()).filter(i=>(i.version||i.sequence)<=e),a={};for(const i of n)i.state&&Object.assign(a,i.state);return{version:e,events:n,places:a}},async loginAs(e){const t=typeof e=="string"?[e]:e,a=await(await fetch(`${f}/api/debug/login`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({login:"pilot-user",roles:t})})).json();return q(a),await this.waitForRender(100),a},logout(){return A(),this.waitForRender()},getRoles(){return(c==null?void 0:c.roles)||[]},hasRole(e){return this.getRoles().includes(e)},assertState(e){const t=this.getStatus();if(t!==e)throw new Error(`Expected state '${e}', got '${t}'`);return this},assertEnabled(e){if(!this.isEnabled(e)){const t=this.getEnabled();throw new Error(`Expected '${e}' to be enabled. Enabled: [${t.join(", ")}]`)}return this},assertDisabled(e){if(this.isEnabled(e))throw new Error(`Expected '${e}' to be disabled, but it is enabled`);return this},assertExists(e){if(!this.exists(e))throw new Error(`Expected element '${e}' to exist`);return this},assertText(e,t){const n=this.getText(e);if(n!==t)throw new Error(`Expected '${e}' to contain '${t}', got '${n}'`);return this},assertAuthenticated(){if(!this.isAuthenticated())throw new Error("Expected user to be authenticated");return this},assertRole(e){if(!this.hasRole(e))throw new Error(`Expected user to have role '${e}'. Has: [${this.getRoles().join(", ")}]`);return this},getTransitions(){return[{id:"start",name:"Start",description:"Start working on a task"},{id:"submit",name:"Submit",description:"Submit task for review"},{id:"approve",name:"Approve",description:"Approve completed task"},{id:"reject",name:"Reject",description:"Reject task and send back"}]},getPlaces(){return[{id:"pending",name:"Pending",initial:1},{id:"in_progress",name:"InProgress",initial:0},{id:"review",name:"Review",initial:0},{id:"completed",name:"Completed",initial:0}]},getTransition(e){return this.getTransitions().find(t=>t.id===e)||null},canFire(e){if(!this.getTransition(e))return{canFire:!1,reason:`Unknown transition: ${e}`};if(!s)return{canFire:!1,reason:"No current instance"};if(!this.isEnabled(e)){const a=this.getStatus();return{canFire:!1,reason:`Transition '${e}' not enabled in state '${a}'`,currentState:a,enabledTransitions:this.getEnabled()}}return{canFire:!0}},async sequence(e,t={}){const n=[],{stopOnError:a=!0,data:i={}}=t;for(const o of e){const d=this.canFire(o);if(!d.canFire){if(a)throw new Error(`Sequence failed at '${o}': ${d.reason}`);n.push({transition:o,success:!1,error:d.reason});continue}try{const r=await this.action(o,i[o]||{});n.push({transition:o,success:!0,state:r.state})}catch(r){if(a)throw r;n.push({transition:o,success:!1,error:r.message})}}return n},getWorkflowInfo(){var e;return{places:this.getPlaces(),transitions:this.getTransitions(),initialPlace:(e=this.getPlaces().find(t=>t.initial>0))==null?void 0:e.id}}};be();Y();
