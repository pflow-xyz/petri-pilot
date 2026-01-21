(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const i of document.querySelectorAll('link[rel="modulepreload"]'))o(i);new MutationObserver(i=>{for(const a of i)if(a.type==="childList")for(const c of a.addedNodes)c.tagName==="LINK"&&c.rel==="modulepreload"&&o(c)}).observe(document,{childList:!0,subtree:!0});function n(i){const a={};return i.integrity&&(a.integrity=i.integrity),i.referrerPolicy&&(a.referrerPolicy=i.referrerPolicy),i.crossOrigin==="use-credentials"?a.credentials="include":i.crossOrigin==="anonymous"?a.credentials="omit":a.credentials="same-origin",a}function o(i){if(i.ep)return;i.ep=!0;const a=n(i);fetch(i.href,a)}})();const D=[{path:"/",component:"List",title:"support-ticket"},{path:"/supportticket",component:"List",title:"support-ticket"},{path:"/supportticket/new",component:"Form",title:"New support-ticket"},{path:"/supportticket/:id",component:"Detail",title:"support-ticket Detail"},{path:"/admin",component:"AdminDashboard",title:"Admin Dashboard"},{path:"/admin/instances",component:"AdminInstances",title:"Instances"},{path:"/admin/instances/:id",component:"AdminInstance",title:"Instance Detail"}];let y=null,$={};function L(e){e=e||"/",e!=="/"&&e.endsWith("/")&&(e=e.slice(0,-1));for(const t of D){const n={};let o=t.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");o=o.replace(/:[^/]+/g,"([^/]+)");const i=new RegExp(`^${o}$`),a=e.match(i);if(a)return(t.path.match(/:[^/]+/g)||[]).map(r=>r.slice(1)).forEach((r,h)=>{n[r]=decodeURIComponent(a[h+1])}),{route:t,params:n}}return null}function u(e,t={}){e.startsWith("/")||(e="/"+e);const n=L(e);if(!n){console.warn(`No route found for path: ${e}, falling back to list`),e="/supportticket";const o=L(e);o&&(y=o.route,$=o.params,window.history.pushState(t,"",e),P());return}if(n.route.roles&&n.route.roles.length>0){const o=Q();if(!o||!X(o,n.route.roles)){console.warn("Access denied:",e),u("/supportticket");return}}y=n.route,$=n.params,window.history.pushState(t,"",e),P()}window.addEventListener("popstate",()=>{const e=window.location.pathname,t=L(e);t?(y=t.route,$=t.params,P()):u("/supportticket")});function Q(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function X(e,t){return!e||!e.roles?!1:t.some(n=>e.roles.includes(n))}function P(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:y,params:$}}))}function Z(){return $}function S(){return y}function ee(){const e=window.location.pathname,t=L(e);t?(y=t.route,$=t.params):(y=D.find(n=>n.path==="/supportticket")||D[0],$={})}const W=[{id:"customer",description:"End user who submits support tickets"},{id:"agent",description:"First-line support agent who handles tickets"},{id:"supervisor",description:"Senior support who handles escalations"},{id:"admin",description:"Full access to all operations"}],N={brand:"support-ticket",items:[{label:"support-ticket",path:"/supportticket",icon:""},{label:"New",path:"/supportticket/new",icon:"+"},{label:"Admin",path:"/admin",icon:""}]};let l=null,M=!1;async function U(){if(!M){M=!0;try{const e={},t=G();t&&(e.Authorization=`Bearer ${t}`);const n=await fetch("/api/navigation",{headers:e});n.ok?l=await n.json():l=N}catch{l=N}finally{M=!1}}}async function K(){l||await U();const e=window.location.pathname,t=ne(),n=(l==null?void 0:l.items)||N.items;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="/supportticket" onclick="handleNavClick(event, '/supportticket')">
          ${(l==null?void 0:l.brand)||N.brand}
        </a>
      </div>
      <ul class="nav-menu">
        ${n.map(a=>`
            <li class="${e===a.path||a.path!=="/"&&e.startsWith(a.path)?"active":""}">
              <a href="${a.path}" onclick="handleNavClick(event, '${a.path}')">
                ${a.icon?`<span class="icon">${a.icon}</span>`:""}
                ${a.label}
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
  `}function te(){return W.length===0?`
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
            ${W.map(t=>`
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
  `}window.showLoginModal=function(){const e=document.getElementById("login-modal");e&&(e.style.display="flex")};window.hideLoginModal=function(){const e=document.getElementById("login-modal");e&&(e.style.display="none")};window.handleRoleLogin=async function(e){try{const t=await fetch("/api/debug/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({login:"pilot-user",roles:e})});if(!t.ok)throw new Error("Login failed");const n=await t.json();localStorage.setItem("auth",JSON.stringify(n)),hideLoginModal(),l=null,window.dispatchEvent(new CustomEvent("auth-change")),await I()}catch(t){console.error("Login error:",t),alert("Login failed. Please try again.")}};window.handleNavClick=function(e,t){e.preventDefault(),u(t)};window.handleLogout=async function(){try{const e=G();e&&await fetch("/auth/logout",{method:"POST",headers:{Authorization:`Bearer ${e}`}})}catch(e){console.error("Logout error:",e)}localStorage.removeItem("auth"),l=null,window.dispatchEvent(new CustomEvent("auth-change")),await I(),u("/supportticket")};function ne(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function G(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).token}catch{return null}return null}async function I(){l=null,await U();const e=document.getElementById("nav");e&&(e.innerHTML=await K())}window.addEventListener("auth-change",async()=>{await I()});window.addEventListener("route-change",()=>{const e=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(t=>{t.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(t=>{const n=t.getAttribute("href");(n===e||n!=="/"&&e.startsWith(n))&&t.parentElement.classList.add("active")})});let J=[];async function oe(){try{const e=await fetch("/api/views");return e.ok?(J=await e.json(),J):(console.warn("Failed to load view definitions, using defaults"),[])}catch(e){return console.error("Error loading views:",e),[]}}const g="",se="";function ae(e){return["amount","value","balance","total_supply","allowance"].some(n=>e.toLowerCase().includes(n))}let d=null,m=null,b=[],s=null;const R=[{id:"assign",name:"Assign",description:"Assign ticket to an agent",fields:[]},{id:"start_work",name:"Start Work",description:"Begin working on the ticket",fields:[]},{id:"escalate",name:"Escalate",description:"Escalate to senior support",fields:[]},{id:"request_info",name:"Request Info",description:"Request more information from customer",fields:[]},{id:"customer_reply",name:"Customer Reply",description:"Customer provides requested information",fields:[]},{id:"resolve",name:"Resolve",description:"Mark issue as resolved from in_progress",fields:[]},{id:"resolve_escalated",name:"Resolve Escalated",description:"Mark escalated issue as resolved",fields:[]},{id:"close",name:"Close",description:"Close the ticket",fields:[]},{id:"reopen",name:"Reopen",description:"Customer reopens a closed ticket",fields:[]}];function ie(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);if(t.expires_at&&new Date(t.expires_at)>new Date)return m=t.token,d=t.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function q(e){localStorage.setItem("auth",JSON.stringify(e)),m=e.token,d=e.user,window.dispatchEvent(new CustomEvent("auth-change"))}function A(){localStorage.removeItem("auth"),m=null,d=null,window.dispatchEvent(new CustomEvent("auth-change"))}function re(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);return m=t.token,d=t.user,!0}catch{return!1}return m=null,d=null,!1}window.addEventListener("auth-change",()=>{re()});function w(){const e={"Content-Type":"application/json"};return m&&(e.Authorization=`Bearer ${m}`),e}async function k(e){if(e.status===401)throw A(),E("Session expired. Please log in again."),new Error("Unauthorized");if(!e.ok){const t=await e.json().catch(()=>({}));throw new Error(t.message||e.statusText)}return e.json()}const p={async getMe(){const e=await fetch(`${g}/auth/me`,{headers:w()});return k(e)},async logout(){await fetch(`${g}/auth/logout`,{method:"POST",headers:w()}),A()},async listInstances(){const e=await fetch(`${g}/admin/instances`,{headers:w()});return k(e)},async getInstance(e){const t=await fetch(`${g}/api/supportticket/${e}`,{headers:w()});return k(t)},async createInstance(e={}){const t=await fetch(`${g}/api/supportticket`,{method:"POST",headers:w(),body:JSON.stringify(e)});return k(t)},async executeTransition(e,t,n={}){const o=n,i=await fetch(`${g}/api/${e}`,{method:"POST",headers:w(),body:JSON.stringify({aggregate_id:t,data:o})});return k(i)}};window.api=p;Object.defineProperty(window,"currentInstance",{get:function(){return s}});window.setAuthToken=function(e){m=e};window.saveAuth=q;window.clearAuth=A;function E(e){const t=document.getElementById("app"),n=t.querySelector(".alert-error");n&&n.remove();const o=document.createElement("div");o.className="alert alert-error",o.textContent=e,t.insertBefore(o,t.firstChild),setTimeout(()=>o.remove(),5e3)}function _(e){const t=document.getElementById("app"),n=t.querySelector(".alert-success");n&&n.remove();const o=document.createElement("div");o.className="alert alert-success",o.textContent=e,t.insertBefore(o,t.firstChild),setTimeout(()=>o.remove(),3e3)}function H(e){if(!e)return"unknown";for(const[t,n]of Object.entries(e))if(n>0)return t;return"unknown"}function z(e){return`<span class="badge ${`badge-${e.toLowerCase().replace(/_/g,"-")}`}">${e.replace(/_/g," ")}</span>`}async function O(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>support-ticket</h1>
        <button class="btn btn-primary" onclick="handleCreateNew()">+ New</button>
      </div>
      <div id="instances-list" class="entity-list">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{b=(await p.listInstances()).instances||[],ce()}catch{document.getElementById("instances-list").innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `}}function ce(){const e=document.getElementById("instances-list");if(e){if(b.length===0){e.innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `;return}e.innerHTML=b.map(t=>{const n=H(t.state||t.places);return`
      <div class="entity-card" onclick="navigate('/supportticket/${t.id}')">
        <div class="entity-info">
          <h3>${t.id}</h3>
          <div class="entity-meta">
            ${z(n)} &middot; Version ${t.version||0}
          </div>
        </div>
        <div class="entity-actions">
          <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); navigate('/supportticket/${t.id}')">
            View
          </button>
        </div>
      </div>
    `}).join("")}}async function de(){const t=Z().id,n=document.getElementById("app");n.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/supportticket')" style="margin-left: -0.5rem">
            &larr; Back to List
          </button>
          <h1 style="margin-top: 0.5rem">Instance: ${t}</h1>
        </div>
      </div>
      <div id="instance-detail">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{const o=await p.getInstance(t);s={id:o.aggregate_id||t,version:o.version,state:o.state,displayState:o.state,places:o.places,enabled:o.enabled||o.enabled_transitions||[]},window.currentInstanceState=s.state,x()}catch(o){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${o.message}</div>
    `}}function x(){const e=document.getElementById("instance-detail");if(!e||!s)return;const t=H(s.places),n=s.enabled||[],o=R;e.innerHTML=`
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
        ${o.map(i=>{const a=n.includes(i.id);return`
            <button
              class="btn ${a?"btn-primary":"btn-secondary"}"
              onclick="handleTransition('${i.id}')"
              ${a?"":"disabled"}
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
  `}function le(e){return!e||Object.keys(e).length===0?'<p style="color: #999;">No state data</p>':Object.entries(e).map(([t,n])=>{if(typeof n=="object"&&n!==null){const o=Object.entries(n);return o.length===0?`
          <div class="detail-field">
            <dt>${j(t)}</dt>
            <dd><span style="color: #999;">Empty</span></dd>
          </div>
        `:`
        <div class="detail-field">
          <dt>${j(t)}</dt>
          <dd>
            <div class="nested-state">
              ${o.map(([i,a])=>{if(typeof a=="object"&&a!==null){const c=Object.entries(a);return c.length===0?`
                      <div class="state-entry">
                        <span class="state-key">${i}</span>
                        <span class="state-value" style="color: #999;">Empty</span>
                      </div>
                    `:`
                    <div class="state-entry nested-group">
                      <span class="state-key">${i}</span>
                      <div class="nested-state" style="margin-left: 1rem;">
                        ${c.map(([r,h])=>`
                          <div class="state-entry">
                            <span class="state-key">${r}</span>
                            <span class="state-value">${B(t,h)}</span>
                          </div>
                        `).join("")}
                      </div>
                    </div>
                  `}return`
                  <div class="state-entry">
                    <span class="state-key">${i}</span>
                    <span class="state-value">${B(t,a)}</span>
                  </div>
                `}).join("")}
            </div>
          </dd>
        </div>
      `}return`
      <div class="detail-field">
        <dt>${j(t)}</dt>
        <dd>${B(t,n)}</dd>
      </div>
    `}).join("")}function j(e){return e.replace(/_/g," ").replace(/\b\w/g,t=>t.toUpperCase())}function B(e,t){return ae(e),`<strong>${t}</strong>`}function ue(){if(typeof wallet<"u"&&wallet.getAccount){const e=wallet.getAccount();return(e==null?void 0:e.address)||null}return null}function pe(e,t){return e?e==="wallet"?ue()||"":e==="user"?(d==null?void 0:d.id)||(d==null?void 0:d.login)||"":(e.startsWith("balances.")||e.includes("."),""):""}function me(){return`
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
  `}let T=null;function he(e){const t=R.find(o=>o.id===e);if(!t)return;T=e,document.getElementById("action-modal-title").textContent=t.name;const n=t.fields.map(o=>{const a=pe(o.autoFill,s==null?void 0:s.state)||o.defaultValue||"",c=o.required?"required":"";let r="";if(o.type==="amount")r=`
        <input
          type="number"
          name="${o.name}"
          value="${a}"
          placeholder="${o.placeholder||"Amount"}"
          step="any"
          ${c}
          class="form-control"
        />
        
      `;else if(o.type==="address"){const h=fe();h.length>0?r=`
          <select name="${o.name}" ${c} class="form-control">
            <option value="">Select address...</option>
            ${h.map(f=>`
              <option value="${f.address}" ${f.address===a?"selected":""}>
                ${f.name||"Account"} (${f.address.slice(0,8)}...${f.address.slice(-6)})
              </option>
            `).join("")}
          </select>
        `:r=`
          <input
            type="text"
            name="${o.name}"
            value="${a}"
            placeholder="${o.placeholder||"0x..."}"
            ${c}
            class="form-control"
          />
        `}else o.type==="hidden"?r=`<input type="hidden" name="${o.name}" value="${a}" />`:r=`
        <input
          type="${o.type==="number"?"number":"text"}"
          name="${o.name}"
          value="${a}"
          placeholder="${o.placeholder||""}"
          ${c}
          class="form-control"
        />
      `;return o.type==="hidden"?r:`
      <div class="form-field">
        <label>${o.label}${o.required?" *":""}</label>
        ${r}
      </div>
    `}).join("");document.getElementById("action-form-fields").innerHTML=n,document.getElementById("action-modal").style.display="flex"}window.hideActionModal=function(){document.getElementById("action-modal").style.display="none",T=null};window.handleActionSubmit=async function(e){var c;if(e.preventDefault(),!T||!s)return;const t=T,n=s.id,o=e.target,i=new FormData(o),a={};for(const[r,h]of i.entries()){const f=(c=R.find(F=>F.id===t))==null?void 0:c.fields.find(F=>F.name===r);f&&(f.type==="amount"||f.type==="number")?a[r]=parseFloat(h)||0:a[r]=h}hideActionModal();try{const r=await p.executeTransition(t,n,a);s={...s,version:r.version,state:r.state,displayState:r.state,places:r.state,enabled:r.enabled||[]},window.currentInstanceState=s.state,x(),_(`Action "${t}" completed!`)}catch(r){E(`Failed to execute ${t}: ${r.message}`)}};function fe(){return typeof wallet<"u"&&wallet.getAccounts?wallet.getAccounts()||[]:[]}window.showAddressPicker=function(e){if(typeof wallet>"u"||!wallet.getAccounts)return;const t=wallet.getAccounts();if(!t||t.length===0)return;const n=document.querySelector(".address-picker-dropdown");n&&n.remove();const o=document.querySelector(`[name="${e}"]`);if(!o)return;const i=o.getBoundingClientRect(),a=document.createElement("div");a.className="address-picker-dropdown",a.style.cssText=`
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
  `,a.innerHTML=t.map(c=>`
    <div class="address-picker-option" onclick="selectAddress('${e}', '${c.address}')" style="
      padding: 8px 12px;
      cursor: pointer;
      border-bottom: 1px solid #eee;
    ">
      <div style="font-weight: 500;">${c.name||"Account"}</div>
      <div style="font-size: 0.85rem; color: #666; font-family: monospace;">${c.address.slice(0,10)}...${c.address.slice(-8)}</div>
    </div>
  `).join(""),document.body.appendChild(a),setTimeout(()=>{document.addEventListener("click",function c(r){a.contains(r.target)||(a.remove(),document.removeEventListener("click",c))})},0)};window.selectAddress=function(e,t){const n=document.querySelector(`[name="${e}"]`);n&&(n.value=t,n.dispatchEvent(new Event("input",{bubbles:!0})));const o=document.querySelector(".address-picker-dropdown");o&&o.remove()};async function ge(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/supportticket')" style="margin-left: -0.5rem">
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
            <button type="button" class="btn btn-secondary" onclick="navigate('/supportticket')">Cancel</button>
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
  `;try{const[t,n]=await Promise.all([fetch(`${g}/admin/stats`,{headers:w()}).then(i=>i.json()).catch(()=>null),p.listInstances()]);t?document.getElementById("admin-stats").innerHTML=`
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
      `:document.getElementById("admin-stats").innerHTML="",b=n.instances||[];const o=document.getElementById("admin-instances").querySelector(".loading");o&&(o.outerHTML=b.length>0?`<table class="table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Status</th>
                <th>Version</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              ${b.slice(0,20).map(i=>{const a=H(i.state||i.places);return`
                  <tr>
                    <td><code>${i.id}</code></td>
                    <td>${z(a)}</td>
                    <td>${i.version||0}</td>
                    <td><button class="btn btn-sm btn-link" onclick="navigate('/supportticket/${i.id}')">View</button></td>
                  </tr>
                `}).join("")}
            </tbody>
          </table>`:'<p style="color: #666; padding: 1rem;">No instances yet.</p>')}catch(t){E("Failed to load admin data: "+t.message)}}window.navigate=u;window.handleCreateNew=async function(){u("/supportticket/new")};window.handleSubmitCreate=async function(e){e.preventDefault();try{const t=await p.createInstance({});_("Instance created successfully!"),u(`/supportticket/${t.aggregate_id||t.id}`)}catch(t){E("Failed to create: "+t.message)}};window.handleTransition=async function(e){if(!s)return;const t=R.find(n=>n.id===e);if(t&&t.fields&&t.fields.length>0){he(e);return}try{const n=await p.executeTransition(e,s.id);s={...s,version:n.version,state:n.state,displayState:n.state,places:n.state,enabled:n.enabled||[]},window.currentInstanceState=s.state,x(),_(`Action "${e}" completed!`)}catch(n){E(`Failed to execute ${e}: ${n.message}`)}};function V(e){var o;const t=((o=e.detail)==null?void 0:o.route)||S();if(!t){O();return}const n=t.path;n==="/supportticket"||n==="/"?O():n==="/supportticket/new"?ge():n==="/supportticket/:id"?de():n==="/admin"||n.startsWith("/admin")?we():O()}async function ve(){const e=new URLSearchParams(window.location.search),t=e.get("token"),n=e.get("expires_at");if(t){m=t;try{const o=await p.getMe();q({token:t,expires_at:n,user:o}),window.history.replaceState({},"",window.location.pathname),await I()}catch{A(),E("Failed to complete login")}}}async function be(){ie(),await ve(),await oe();const e=document.getElementById("nav");e.innerHTML=await K();const t=document.createElement("div");t.innerHTML=me(),document.body.appendChild(t),window.addEventListener("route-change",V),ee(),V({detail:{route:S()}})}let v=null,C=null;function Y(){const t=`${window.location.protocol==="https:"?"wss:":"ws:"}//${window.location.host}/ws`;v=new WebSocket(t),v.onopen=()=>{console.log("[Debug] WebSocket connected")},v.onmessage=n=>{try{const o=JSON.parse(n.data);o.id==="session"&&o.type==="session"?(C=(typeof o.data=="string"?JSON.parse(o.data):o.data).session_id,console.log("[Debug] Session ID:",C)):o.type==="eval"&&ye(o)}catch(o){console.error("[Debug] Failed to parse message:",o)}},v.onclose=()=>{console.log("[Debug] WebSocket disconnected, reconnecting in 3s..."),C=null,setTimeout(Y,3e3)},v.onerror=n=>{console.error("[Debug] WebSocket error:",n)}}async function ye(e){try{const n=(typeof e.data=="string"?JSON.parse(e.data):e.data).code,i=await new Function("return (async () => { "+n+" })()")(),a={type:"response",id:e.id,data:{result:i,type:typeof i}};v.send(JSON.stringify(a))}catch(t){const n={type:"response",id:e.id,data:{error:t.message}};v.send(JSON.stringify(n))}}window.debugSessionId=()=>C;window.debugWs=()=>v;window.pilot={async list(){return u("/supportticket"),await this.waitFor(".entity-card, .empty-state",5e3).catch(()=>{}),b},newForm(){return u("/supportticket/new"),this.waitForRender()},async view(e){return u(`/supportticket/${e}`),await this.waitForRender(),s},admin(){return u("/admin"),this.waitForRender()},async create(e={}){const t=await p.createInstance(e),n=t.aggregate_id||t.id;return u(`/supportticket/${n}`),await this.waitForRender(),{id:n,...t}},getCurrentInstance(){return s},getInstances(){return b},async refresh(){if(!s)throw new Error("No current instance");const e=await p.getInstance(s.id);return s={id:e.aggregate_id||s.id,version:e.version,state:e.state,places:e.places,enabled:e.enabled||e.enabled_transitions||[]},x(),s},async action(e,t={}){if(!s)throw new Error("No current instance - navigate to detail page first");const n=await p.executeTransition(e,s.id,t);return s={...s,version:n.version,state:n.state,places:n.state,enabled:n.enabled||[]},x(),{success:!0,state:s.places,enabled:s.enabled}},isEnabled(e){return s?(s.enabled||[]).includes(e):!1},getEnabled(){return(s==null?void 0:s.enabled)||[]},fill(e,t){const n=document.querySelector(`[name="${e}"]`);if(!n)throw new Error(`No input found with name: ${e}`);return n.value=t,n.dispatchEvent(new Event("input",{bubbles:!0})),this},async submit(){const e=document.querySelector("form");if(!e)throw new Error("No form found on page");const t=new Event("submit",{bubbles:!0,cancelable:!0});return e.dispatchEvent(t),await this.waitForRender(),s},getText(e){const t=document.querySelector(e);return t?t.textContent.trim():null},exists(e){return document.querySelector(e)!==null},getButtons(){return Array.from(document.querySelectorAll("button")).map(e=>({text:e.textContent.trim(),disabled:e.disabled,className:e.className}))},async clickButton(e){const t=document.querySelectorAll("button");for(const n of t)if(n.textContent.trim()===e&&!n.disabled)return n.click(),await this.waitForRender(),!0;throw new Error(`No enabled button found with text: ${e}`)},getState(){return(s==null?void 0:s.places)||null},getStatus(){if(!(s!=null&&s.places))return null;for(const[e,t]of Object.entries(s.places))if(t>0)return e;return null},getRoute(){return S()},getUser(){return d},isAuthenticated(){return m!==null},waitForRender(e=50){return new Promise(t=>setTimeout(t,e))},async waitFor(e,t=5e3){const n=Date.now();for(;Date.now()-n<t;){if(document.querySelector(e))return document.querySelector(e);await this.waitForRender(50)}throw new Error(`Timeout waiting for: ${e}`)},async waitForState(e,t=5e3){var o;const n=Date.now();for(;Date.now()-n<t;){if(((o=s==null?void 0:s.places)==null?void 0:o[e])>0)return s;await this.waitForRender(100)}throw new Error(`Timeout waiting for state: ${e}`)},debug(){return console.log("=== Pilot Debug ==="),console.log("Route:",S()),console.log("User:",d),console.log("Instance:",s),console.log("Enabled:",s==null?void 0:s.enabled),console.log("State:",s==null?void 0:s.places),{route:S(),user:d,instance:s}},async getEvents(){if(!s)throw new Error("No current instance");const e=await fetch(`${g}/api/supportticket/${s.id}/events`,{headers:w()});return(await k(e)).events||[]},async getEventCount(){return(await this.getEvents()).length},async getLastEvent(){const e=await this.getEvents();return e.length>0?e[e.length-1]:null},async replayTo(e){if(!s)throw new Error("No current instance");const n=(await this.getEvents()).filter(i=>(i.version||i.sequence)<=e),o={};for(const i of n)i.state&&Object.assign(o,i.state);return{version:e,events:n,places:o}},async loginAs(e){const t=typeof e=="string"?[e]:e,o=await(await fetch(`${g}/api/debug/login`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({login:"pilot-user",roles:t})})).json();return q(o),await this.waitForRender(100),o},logout(){return A(),this.waitForRender()},getRoles(){return(d==null?void 0:d.roles)||[]},hasRole(e){return this.getRoles().includes(e)},assertState(e){const t=this.getStatus();if(t!==e)throw new Error(`Expected state '${e}', got '${t}'`);return this},assertEnabled(e){if(!this.isEnabled(e)){const t=this.getEnabled();throw new Error(`Expected '${e}' to be enabled. Enabled: [${t.join(", ")}]`)}return this},assertDisabled(e){if(this.isEnabled(e))throw new Error(`Expected '${e}' to be disabled, but it is enabled`);return this},assertExists(e){if(!this.exists(e))throw new Error(`Expected element '${e}' to exist`);return this},assertText(e,t){const n=this.getText(e);if(n!==t)throw new Error(`Expected '${e}' to contain '${t}', got '${n}'`);return this},assertAuthenticated(){if(!this.isAuthenticated())throw new Error("Expected user to be authenticated");return this},assertRole(e){if(!this.hasRole(e))throw new Error(`Expected user to have role '${e}'. Has: [${this.getRoles().join(", ")}]`);return this},getTransitions(){return[{id:"assign",name:"Assign",description:"Assign ticket to an agent"},{id:"start_work",name:"Start Work",description:"Begin working on the ticket"},{id:"escalate",name:"Escalate",description:"Escalate to senior support"},{id:"request_info",name:"Request Info",description:"Request more information from customer"},{id:"customer_reply",name:"Customer Reply",description:"Customer provides requested information"},{id:"resolve",name:"Resolve",description:"Mark issue as resolved from in_progress"},{id:"resolve_escalated",name:"Resolve Escalated",description:"Mark escalated issue as resolved"},{id:"close",name:"Close",description:"Close the ticket"},{id:"reopen",name:"Reopen",description:"Customer reopens a closed ticket"}]},getPlaces(){return[{id:"new",name:"New",initial:1},{id:"assigned",name:"Assigned",initial:0},{id:"in_progress",name:"InProgress",initial:0},{id:"escalated",name:"Escalated",initial:0},{id:"pending_customer",name:"PendingCustomer",initial:0},{id:"resolved",name:"Resolved",initial:0},{id:"closed",name:"Closed",initial:0}]},getTransition(e){return this.getTransitions().find(t=>t.id===e)||null},canFire(e){if(!this.getTransition(e))return{canFire:!1,reason:`Unknown transition: ${e}`};if(!s)return{canFire:!1,reason:"No current instance"};if(!this.isEnabled(e)){const o=this.getStatus();return{canFire:!1,reason:`Transition '${e}' not enabled in state '${o}'`,currentState:o,enabledTransitions:this.getEnabled()}}return{canFire:!0}},async sequence(e,t={}){const n=[],{stopOnError:o=!0,data:i={}}=t;for(const a of e){const c=this.canFire(a);if(!c.canFire){if(o)throw new Error(`Sequence failed at '${a}': ${c.reason}`);n.push({transition:a,success:!1,error:c.reason});continue}try{const r=await this.action(a,i[a]||{});n.push({transition:a,success:!0,state:r.state})}catch(r){if(o)throw r;n.push({transition:a,success:!1,error:r.message})}}return n},getWorkflowInfo(){var e;return{places:this.getPlaces(),transitions:this.getTransitions(),initialPlace:(e=this.getPlaces().find(t=>t.initial>0))==null?void 0:e.id}}};be();Y();
