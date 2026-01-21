(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const s of document.querySelectorAll('link[rel="modulepreload"]'))a(s);new MutationObserver(s=>{for(const i of s)if(i.type==="childList")for(const c of i.addedNodes)c.tagName==="LINK"&&c.rel==="modulepreload"&&a(c)}).observe(document,{childList:!0,subtree:!0});function n(s){const i={};return s.integrity&&(i.integrity=s.integrity),s.referrerPolicy&&(i.referrerPolicy=s.referrerPolicy),s.crossOrigin==="use-credentials"?i.credentials="include":s.crossOrigin==="anonymous"?i.credentials="omit":i.credentials="same-origin",i}function a(s){if(s.ep)return;s.ep=!0;const i=n(s);fetch(s.href,i)}})();const M=[{path:"/",component:"List",title:"ecommerce-checkout"},{path:"/ecommercecheckout",component:"List",title:"ecommerce-checkout"},{path:"/ecommercecheckout/new",component:"Form",title:"New ecommerce-checkout"},{path:"/ecommercecheckout/:id",component:"Detail",title:"ecommerce-checkout Detail"},{path:"/admin",component:"AdminDashboard",title:"Admin Dashboard"},{path:"/admin/instances",component:"AdminInstances",title:"Instances"},{path:"/admin/instances/:id",component:"AdminInstance",title:"Instance Detail"}];let b=null,$={};function C(e){e=e||"/",e!=="/"&&e.endsWith("/")&&(e=e.slice(0,-1));for(const t of M){const n={};let a=t.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");a=a.replace(/:[^/]+/g,"([^/]+)");const s=new RegExp(`^${a}$`),i=e.match(s);if(i)return(t.path.match(/:[^/]+/g)||[]).map(r=>r.slice(1)).forEach((r,h)=>{n[r]=decodeURIComponent(i[h+1])}),{route:t,params:n}}return null}function u(e,t={}){e.startsWith("/")||(e="/"+e);const n=C(e);if(!n){console.warn(`No route found for path: ${e}, falling back to list`),e="/ecommercecheckout";const a=C(e);a&&(b=a.route,$=a.params,window.history.pushState(t,"",e),B());return}if(n.route.roles&&n.route.roles.length>0){const a=Q();if(!a||!X(a,n.route.roles)){console.warn("Access denied:",e),u("/ecommercecheckout");return}}b=n.route,$=n.params,window.history.pushState(t,"",e),B()}window.addEventListener("popstate",()=>{const e=window.location.pathname,t=C(e);t?(b=t.route,$=t.params,B()):u("/ecommercecheckout")});function Q(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function X(e,t){return!e||!e.roles?!1:t.some(n=>e.roles.includes(n))}function B(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:b,params:$}}))}function Z(){return $}function S(){return b}function ee(){const e=window.location.pathname,t=C(e);t?(b=t.route,$=t.params):(b=M.find(n=>n.path==="/ecommercecheckout")||M[0],$={})}const W=[{id:"customer",description:"End user making a purchase"},{id:"system",description:"Automated payment processing system"},{id:"fulfillment",description:"Warehouse staff who fulfill orders"},{id:"admin",description:"Full access to all operations"}],T={brand:"ecommerce-checkout",items:[{label:"ecommerce-checkout",path:"/ecommercecheckout",icon:""},{label:"New",path:"/ecommercecheckout/new",icon:"+"},{label:"Admin",path:"/admin",icon:""}]};let l=null,R=!1;async function U(){if(!R){R=!0;try{const e={},t=G();t&&(e.Authorization=`Bearer ${t}`);const n=await fetch("/api/navigation",{headers:e});n.ok?l=await n.json():l=T}catch{l=T}finally{R=!1}}}async function K(){l||await U();const e=window.location.pathname,t=ne(),n=(l==null?void 0:l.items)||T.items;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="/ecommercecheckout" onclick="handleNavClick(event, '/ecommercecheckout')">
          ${(l==null?void 0:l.brand)||T.brand}
        </a>
      </div>
      <ul class="nav-menu">
        ${n.map(i=>`
            <li class="${e===i.path||i.path!=="/"&&e.startsWith(i.path)?"active":""}">
              <a href="${i.path}" onclick="handleNavClick(event, '${i.path}')">
                ${i.icon?`<span class="icon">${i.icon}</span>`:""}
                ${i.label}
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
  `}window.showLoginModal=function(){const e=document.getElementById("login-modal");e&&(e.style.display="flex")};window.hideLoginModal=function(){const e=document.getElementById("login-modal");e&&(e.style.display="none")};window.handleRoleLogin=async function(e){try{const t=await fetch("/api/debug/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({login:"pilot-user",roles:e})});if(!t.ok)throw new Error("Login failed");const n=await t.json();localStorage.setItem("auth",JSON.stringify(n)),hideLoginModal(),l=null,window.dispatchEvent(new CustomEvent("auth-change")),await P()}catch(t){console.error("Login error:",t),alert("Login failed. Please try again.")}};window.handleNavClick=function(e,t){e.preventDefault(),u(t)};window.handleLogout=async function(){try{const e=G();e&&await fetch("/auth/logout",{method:"POST",headers:{Authorization:`Bearer ${e}`}})}catch(e){console.error("Logout error:",e)}localStorage.removeItem("auth"),l=null,window.dispatchEvent(new CustomEvent("auth-change")),await P(),u("/ecommercecheckout")};function ne(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function G(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).token}catch{return null}return null}async function P(){l=null,await U();const e=document.getElementById("nav");e&&(e.innerHTML=await K())}window.addEventListener("auth-change",async()=>{await P()});window.addEventListener("route-change",()=>{const e=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(t=>{t.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(t=>{const n=t.getAttribute("href");(n===e||n!=="/"&&e.startsWith(n))&&t.parentElement.classList.add("active")})});let J=[];async function ae(){try{const e=await fetch("/api/views");return e.ok?(J=await e.json(),J):(console.warn("Failed to load view definitions, using defaults"),[])}catch(e){return console.error("Error loading views:",e),[]}}const g="",oe="";function ie(e){return["amount","value","balance","total_supply","allowance"].some(n=>e.toLowerCase().includes(n))}let d=null,p=null,v=[],o=null;const _=[{id:"start_checkout",name:"Start Checkout",description:"Begin checkout process",fields:[]},{id:"enter_payment",name:"Enter Payment",description:"Enter payment details",fields:[]},{id:"process_payment",name:"Process Payment",description:"Process the payment",fields:[]},{id:"payment_success",name:"Payment Success",description:"Payment processed successfully",fields:[]},{id:"payment_fail_1",name:"Payment Fail 1",description:"First payment attempt failed",fields:[]},{id:"retry_payment_1",name:"Retry Payment 1",description:"Retry payment (attempt 2)",fields:[]},{id:"payment_fail_2",name:"Payment Fail 2",description:"Second payment attempt failed",fields:[]},{id:"retry_payment_2",name:"Retry Payment 2",description:"Retry payment (attempt 3)",fields:[]},{id:"payment_fail_3",name:"Payment Fail 3",description:"Third payment attempt failed",fields:[]},{id:"cancel_order",name:"Cancel Order",description:"Cancel order after max retries",fields:[]},{id:"fulfill",name:"Fulfill",description:"Fulfill the order",fields:[]}];function se(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);if(t.expires_at&&new Date(t.expires_at)>new Date)return p=t.token,d=t.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function D(e){localStorage.setItem("auth",JSON.stringify(e)),p=e.token,d=e.user,window.dispatchEvent(new CustomEvent("auth-change"))}function A(){localStorage.removeItem("auth"),p=null,d=null,window.dispatchEvent(new CustomEvent("auth-change"))}function re(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);return p=t.token,d=t.user,!0}catch{return!1}return p=null,d=null,!1}window.addEventListener("auth-change",()=>{re()});function y(){const e={"Content-Type":"application/json"};return p&&(e.Authorization=`Bearer ${p}`),e}async function k(e){if(e.status===401)throw A(),E("Session expired. Please log in again."),new Error("Unauthorized");if(!e.ok){const t=await e.json().catch(()=>({}));throw new Error(t.message||e.statusText)}return e.json()}const m={async getMe(){const e=await fetch(`${g}/auth/me`,{headers:y()});return k(e)},async logout(){await fetch(`${g}/auth/logout`,{method:"POST",headers:y()}),A()},async listInstances(){const e=await fetch(`${g}/admin/instances`,{headers:y()});return k(e)},async getInstance(e){const t=await fetch(`${g}/api/ecommercecheckout/${e}`,{headers:y()});return k(t)},async createInstance(e={}){const t=await fetch(`${g}/api/ecommercecheckout`,{method:"POST",headers:y(),body:JSON.stringify(e)});return k(t)},async executeTransition(e,t,n={}){const a=n,s=await fetch(`${g}/api/${e}`,{method:"POST",headers:y(),body:JSON.stringify({aggregate_id:t,data:a})});return k(s)}};window.api=m;Object.defineProperty(window,"currentInstance",{get:function(){return o}});window.setAuthToken=function(e){p=e};window.saveAuth=D;window.clearAuth=A;function E(e){const t=document.getElementById("app"),n=t.querySelector(".alert-error");n&&n.remove();const a=document.createElement("div");a.className="alert alert-error",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),5e3)}function q(e){const t=document.getElementById("app"),n=t.querySelector(".alert-success");n&&n.remove();const a=document.createElement("div");a.className="alert alert-success",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),3e3)}function H(e){if(!e)return"unknown";for(const[t,n]of Object.entries(e))if(n>0)return t;return"unknown"}function z(e){return`<span class="badge ${`badge-${e.toLowerCase().replace(/_/g,"-")}`}">${e.replace(/_/g," ")}</span>`}async function I(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>ecommerce-checkout</h1>
        <button class="btn btn-primary" onclick="handleCreateNew()">+ New</button>
      </div>
      <div id="instances-list" class="entity-list">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{v=(await m.listInstances()).instances||[],ce()}catch{document.getElementById("instances-list").innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `}}function ce(){const e=document.getElementById("instances-list");if(e){if(v.length===0){e.innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `;return}e.innerHTML=v.map(t=>{const n=H(t.state||t.places);return`
      <div class="entity-card" onclick="navigate('/ecommercecheckout/${t.id}')">
        <div class="entity-info">
          <h3>${t.id}</h3>
          <div class="entity-meta">
            ${z(n)} &middot; Version ${t.version||0}
          </div>
        </div>
        <div class="entity-actions">
          <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); navigate('/ecommercecheckout/${t.id}')">
            View
          </button>
        </div>
      </div>
    `}).join("")}}async function de(){const t=Z().id,n=document.getElementById("app");n.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/ecommercecheckout')" style="margin-left: -0.5rem">
            &larr; Back to List
          </button>
          <h1 style="margin-top: 0.5rem">Instance: ${t}</h1>
        </div>
      </div>
      <div id="instance-detail">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{const a=await m.getInstance(t);o={id:a.aggregate_id||t,version:a.version,state:a.state,displayState:a.state,places:a.places,enabled:a.enabled||a.enabled_transitions||[]},window.currentInstanceState=o.state,x()}catch(a){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${a.message}</div>
    `}}function x(){const e=document.getElementById("instance-detail");if(!e||!o)return;const t=H(o.places),n=o.enabled||[],a=_;e.innerHTML=`
    <div class="card">
      <div class="card-header">Status</div>
      <div class="detail-list">
        <div class="detail-field">
          <dt>ID</dt>
          <dd><code>${o.id}</code></dd>
        </div>
        <div class="detail-field">
          <dt>Status</dt>
          <dd>${z(t)}</dd>
        </div>
        <div class="detail-field">
          <dt>Version</dt>
          <dd>${o.version||0}</dd>
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
        ${le(o.displayState||o.state)}
      </div>
    </div>
  `}function le(e){return!e||Object.keys(e).length===0?'<p style="color: #999;">No state data</p>':Object.entries(e).map(([t,n])=>{if(typeof n=="object"&&n!==null){const a=Object.entries(n);return a.length===0?`
          <div class="detail-field">
            <dt>${O(t)}</dt>
            <dd><span style="color: #999;">Empty</span></dd>
          </div>
        `:`
        <div class="detail-field">
          <dt>${O(t)}</dt>
          <dd>
            <div class="nested-state">
              ${a.map(([s,i])=>{if(typeof i=="object"&&i!==null){const c=Object.entries(i);return c.length===0?`
                      <div class="state-entry">
                        <span class="state-key">${s}</span>
                        <span class="state-value" style="color: #999;">Empty</span>
                      </div>
                    `:`
                    <div class="state-entry nested-group">
                      <span class="state-key">${s}</span>
                      <div class="nested-state" style="margin-left: 1rem;">
                        ${c.map(([r,h])=>`
                          <div class="state-entry">
                            <span class="state-key">${r}</span>
                            <span class="state-value">${j(t,h)}</span>
                          </div>
                        `).join("")}
                      </div>
                    </div>
                  `}return`
                  <div class="state-entry">
                    <span class="state-key">${s}</span>
                    <span class="state-value">${j(t,i)}</span>
                  </div>
                `}).join("")}
            </div>
          </dd>
        </div>
      `}return`
      <div class="detail-field">
        <dt>${O(t)}</dt>
        <dd>${j(t,n)}</dd>
      </div>
    `}).join("")}function O(e){return e.replace(/_/g," ").replace(/\b\w/g,t=>t.toUpperCase())}function j(e,t){return ie(e),`<strong>${t}</strong>`}function ue(){if(typeof wallet<"u"&&wallet.getAccount){const e=wallet.getAccount();return(e==null?void 0:e.address)||null}return null}function me(e,t){return e?e==="wallet"?ue()||"":e==="user"?(d==null?void 0:d.id)||(d==null?void 0:d.login)||"":(e.startsWith("balances.")||e.includes("."),""):""}function pe(){return`
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
  `}let N=null;function he(e){const t=_.find(a=>a.id===e);if(!t)return;N=e,document.getElementById("action-modal-title").textContent=t.name;const n=t.fields.map(a=>{const i=me(a.autoFill,o==null?void 0:o.state)||a.defaultValue||"",c=a.required?"required":"";let r="";if(a.type==="amount")r=`
        <input
          type="number"
          name="${a.name}"
          value="${i}"
          placeholder="${a.placeholder||"Amount"}"
          step="any"
          ${c}
          class="form-control"
        />
        
      `;else if(a.type==="address"){const h=fe();h.length>0?r=`
          <select name="${a.name}" ${c} class="form-control">
            <option value="">Select address...</option>
            ${h.map(f=>`
              <option value="${f.address}" ${f.address===i?"selected":""}>
                ${f.name||"Account"} (${f.address.slice(0,8)}...${f.address.slice(-6)})
              </option>
            `).join("")}
          </select>
        `:r=`
          <input
            type="text"
            name="${a.name}"
            value="${i}"
            placeholder="${a.placeholder||"0x..."}"
            ${c}
            class="form-control"
          />
        `}else a.type==="hidden"?r=`<input type="hidden" name="${a.name}" value="${i}" />`:r=`
        <input
          type="${a.type==="number"?"number":"text"}"
          name="${a.name}"
          value="${i}"
          placeholder="${a.placeholder||""}"
          ${c}
          class="form-control"
        />
      `;return a.type==="hidden"?r:`
      <div class="form-field">
        <label>${a.label}${a.required?" *":""}</label>
        ${r}
      </div>
    `}).join("");document.getElementById("action-form-fields").innerHTML=n,document.getElementById("action-modal").style.display="flex"}window.hideActionModal=function(){document.getElementById("action-modal").style.display="none",N=null};window.handleActionSubmit=async function(e){var c;if(e.preventDefault(),!N||!o)return;const t=N,n=o.id,a=e.target,s=new FormData(a),i={};for(const[r,h]of s.entries()){const f=(c=_.find(F=>F.id===t))==null?void 0:c.fields.find(F=>F.name===r);f&&(f.type==="amount"||f.type==="number")?i[r]=parseFloat(h)||0:i[r]=h}hideActionModal();try{const r=await m.executeTransition(t,n,i);o={...o,version:r.version,state:r.state,displayState:r.state,places:r.state,enabled:r.enabled||[]},window.currentInstanceState=o.state,x(),q(`Action "${t}" completed!`)}catch(r){E(`Failed to execute ${t}: ${r.message}`)}};function fe(){return typeof wallet<"u"&&wallet.getAccounts?wallet.getAccounts()||[]:[]}window.showAddressPicker=function(e){if(typeof wallet>"u"||!wallet.getAccounts)return;const t=wallet.getAccounts();if(!t||t.length===0)return;const n=document.querySelector(".address-picker-dropdown");n&&n.remove();const a=document.querySelector(`[name="${e}"]`);if(!a)return;const s=a.getBoundingClientRect(),i=document.createElement("div");i.className="address-picker-dropdown",i.style.cssText=`
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
  `,i.innerHTML=t.map(c=>`
    <div class="address-picker-option" onclick="selectAddress('${e}', '${c.address}')" style="
      padding: 8px 12px;
      cursor: pointer;
      border-bottom: 1px solid #eee;
    ">
      <div style="font-weight: 500;">${c.name||"Account"}</div>
      <div style="font-size: 0.85rem; color: #666; font-family: monospace;">${c.address.slice(0,10)}...${c.address.slice(-8)}</div>
    </div>
  `).join(""),document.body.appendChild(i),setTimeout(()=>{document.addEventListener("click",function c(r){i.contains(r.target)||(i.remove(),document.removeEventListener("click",c))})},0)};window.selectAddress=function(e,t){const n=document.querySelector(`[name="${e}"]`);n&&(n.value=t,n.dispatchEvent(new Event("input",{bubbles:!0})));const a=document.querySelector(".address-picker-dropdown");a&&a.remove()};async function ge(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/ecommercecheckout')" style="margin-left: -0.5rem">
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
            <button type="button" class="btn btn-secondary" onclick="navigate('/ecommercecheckout')">Cancel</button>
          </div>
        </form>
      </div>
    </div>
  `}async function ye(){const e=document.getElementById("app");e.innerHTML=`
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
  `;try{const[t,n]=await Promise.all([fetch(`${g}/admin/stats`,{headers:y()}).then(s=>s.json()).catch(()=>null),m.listInstances()]);t?document.getElementById("admin-stats").innerHTML=`
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
      `:document.getElementById("admin-stats").innerHTML="",v=n.instances||[];const a=document.getElementById("admin-instances").querySelector(".loading");a&&(a.outerHTML=v.length>0?`<table class="table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Status</th>
                <th>Version</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              ${v.slice(0,20).map(s=>{const i=H(s.state||s.places);return`
                  <tr>
                    <td><code>${s.id}</code></td>
                    <td>${z(i)}</td>
                    <td>${s.version||0}</td>
                    <td><button class="btn btn-sm btn-link" onclick="navigate('/ecommercecheckout/${s.id}')">View</button></td>
                  </tr>
                `}).join("")}
            </tbody>
          </table>`:'<p style="color: #666; padding: 1rem;">No instances yet.</p>')}catch(t){E("Failed to load admin data: "+t.message)}}window.navigate=u;window.handleCreateNew=async function(){u("/ecommercecheckout/new")};window.handleSubmitCreate=async function(e){e.preventDefault();try{const t=await m.createInstance({});q("Instance created successfully!"),u(`/ecommercecheckout/${t.aggregate_id||t.id}`)}catch(t){E("Failed to create: "+t.message)}};window.handleTransition=async function(e){if(!o)return;const t=_.find(n=>n.id===e);if(t&&t.fields&&t.fields.length>0){he(e);return}try{const n=await m.executeTransition(e,o.id);o={...o,version:n.version,state:n.state,displayState:n.state,places:n.state,enabled:n.enabled||[]},window.currentInstanceState=o.state,x(),q(`Action "${e}" completed!`)}catch(n){E(`Failed to execute ${e}: ${n.message}`)}};function V(e){var a;const t=((a=e.detail)==null?void 0:a.route)||S();if(!t){I();return}const n=t.path;n==="/ecommercecheckout"||n==="/"?I():n==="/ecommercecheckout/new"?ge():n==="/ecommercecheckout/:id"?de():n==="/admin"||n.startsWith("/admin")?ye():I()}async function we(){const e=new URLSearchParams(window.location.search),t=e.get("token"),n=e.get("expires_at");if(t){p=t;try{const a=await m.getMe();D({token:t,expires_at:n,user:a}),window.history.replaceState({},"",window.location.pathname),await P()}catch{A(),E("Failed to complete login")}}}async function ve(){se(),await we(),await ae();const e=document.getElementById("nav");e.innerHTML=await K();const t=document.createElement("div");t.innerHTML=pe(),document.body.appendChild(t),window.addEventListener("route-change",V),ee(),V({detail:{route:S()}})}let w=null,L=null;function Y(){const t=`${window.location.protocol==="https:"?"wss:":"ws:"}//${window.location.host}/ws`;w=new WebSocket(t),w.onopen=()=>{console.log("[Debug] WebSocket connected")},w.onmessage=n=>{try{const a=JSON.parse(n.data);a.id==="session"&&a.type==="session"?(L=(typeof a.data=="string"?JSON.parse(a.data):a.data).session_id,console.log("[Debug] Session ID:",L)):a.type==="eval"&&be(a)}catch(a){console.error("[Debug] Failed to parse message:",a)}},w.onclose=()=>{console.log("[Debug] WebSocket disconnected, reconnecting in 3s..."),L=null,setTimeout(Y,3e3)},w.onerror=n=>{console.error("[Debug] WebSocket error:",n)}}async function be(e){try{const n=(typeof e.data=="string"?JSON.parse(e.data):e.data).code,s=await new Function("return (async () => { "+n+" })()")(),i={type:"response",id:e.id,data:{result:s,type:typeof s}};w.send(JSON.stringify(i))}catch(t){const n={type:"response",id:e.id,data:{error:t.message}};w.send(JSON.stringify(n))}}window.debugSessionId=()=>L;window.debugWs=()=>w;window.pilot={async list(){return u("/ecommercecheckout"),await this.waitFor(".entity-card, .empty-state",5e3).catch(()=>{}),v},newForm(){return u("/ecommercecheckout/new"),this.waitForRender()},async view(e){return u(`/ecommercecheckout/${e}`),await this.waitForRender(),o},admin(){return u("/admin"),this.waitForRender()},async create(e={}){const t=await m.createInstance(e),n=t.aggregate_id||t.id;return u(`/ecommercecheckout/${n}`),await this.waitForRender(),{id:n,...t}},getCurrentInstance(){return o},getInstances(){return v},async refresh(){if(!o)throw new Error("No current instance");const e=await m.getInstance(o.id);return o={id:e.aggregate_id||o.id,version:e.version,state:e.state,places:e.places,enabled:e.enabled||e.enabled_transitions||[]},x(),o},async action(e,t={}){if(!o)throw new Error("No current instance - navigate to detail page first");const n=await m.executeTransition(e,o.id,t);return o={...o,version:n.version,state:n.state,places:n.state,enabled:n.enabled||[]},x(),{success:!0,state:o.places,enabled:o.enabled}},isEnabled(e){return o?(o.enabled||[]).includes(e):!1},getEnabled(){return(o==null?void 0:o.enabled)||[]},fill(e,t){const n=document.querySelector(`[name="${e}"]`);if(!n)throw new Error(`No input found with name: ${e}`);return n.value=t,n.dispatchEvent(new Event("input",{bubbles:!0})),this},async submit(){const e=document.querySelector("form");if(!e)throw new Error("No form found on page");const t=new Event("submit",{bubbles:!0,cancelable:!0});return e.dispatchEvent(t),await this.waitForRender(),o},getText(e){const t=document.querySelector(e);return t?t.textContent.trim():null},exists(e){return document.querySelector(e)!==null},getButtons(){return Array.from(document.querySelectorAll("button")).map(e=>({text:e.textContent.trim(),disabled:e.disabled,className:e.className}))},async clickButton(e){const t=document.querySelectorAll("button");for(const n of t)if(n.textContent.trim()===e&&!n.disabled)return n.click(),await this.waitForRender(),!0;throw new Error(`No enabled button found with text: ${e}`)},getState(){return(o==null?void 0:o.places)||null},getStatus(){if(!(o!=null&&o.places))return null;for(const[e,t]of Object.entries(o.places))if(t>0)return e;return null},getRoute(){return S()},getUser(){return d},isAuthenticated(){return p!==null},waitForRender(e=50){return new Promise(t=>setTimeout(t,e))},async waitFor(e,t=5e3){const n=Date.now();for(;Date.now()-n<t;){if(document.querySelector(e))return document.querySelector(e);await this.waitForRender(50)}throw new Error(`Timeout waiting for: ${e}`)},async waitForState(e,t=5e3){var a;const n=Date.now();for(;Date.now()-n<t;){if(((a=o==null?void 0:o.places)==null?void 0:a[e])>0)return o;await this.waitForRender(100)}throw new Error(`Timeout waiting for state: ${e}`)},debug(){return console.log("=== Pilot Debug ==="),console.log("Route:",S()),console.log("User:",d),console.log("Instance:",o),console.log("Enabled:",o==null?void 0:o.enabled),console.log("State:",o==null?void 0:o.places),{route:S(),user:d,instance:o}},async getEvents(){if(!o)throw new Error("No current instance");const e=await fetch(`${g}/api/ecommercecheckout/${o.id}/events`,{headers:y()});return(await k(e)).events||[]},async getEventCount(){return(await this.getEvents()).length},async getLastEvent(){const e=await this.getEvents();return e.length>0?e[e.length-1]:null},async replayTo(e){if(!o)throw new Error("No current instance");const n=(await this.getEvents()).filter(s=>(s.version||s.sequence)<=e),a={};for(const s of n)s.state&&Object.assign(a,s.state);return{version:e,events:n,places:a}},async loginAs(e){const t=typeof e=="string"?[e]:e,a=await(await fetch(`${g}/api/debug/login`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({login:"pilot-user",roles:t})})).json();return D(a),await this.waitForRender(100),a},logout(){return A(),this.waitForRender()},getRoles(){return(d==null?void 0:d.roles)||[]},hasRole(e){return this.getRoles().includes(e)},assertState(e){const t=this.getStatus();if(t!==e)throw new Error(`Expected state '${e}', got '${t}'`);return this},assertEnabled(e){if(!this.isEnabled(e)){const t=this.getEnabled();throw new Error(`Expected '${e}' to be enabled. Enabled: [${t.join(", ")}]`)}return this},assertDisabled(e){if(this.isEnabled(e))throw new Error(`Expected '${e}' to be disabled, but it is enabled`);return this},assertExists(e){if(!this.exists(e))throw new Error(`Expected element '${e}' to exist`);return this},assertText(e,t){const n=this.getText(e);if(n!==t)throw new Error(`Expected '${e}' to contain '${t}', got '${n}'`);return this},assertAuthenticated(){if(!this.isAuthenticated())throw new Error("Expected user to be authenticated");return this},assertRole(e){if(!this.hasRole(e))throw new Error(`Expected user to have role '${e}'. Has: [${this.getRoles().join(", ")}]`);return this},getTransitions(){return[{id:"start_checkout",name:"Start Checkout",description:"Begin checkout process"},{id:"enter_payment",name:"Enter Payment",description:"Enter payment details"},{id:"process_payment",name:"Process Payment",description:"Process the payment"},{id:"payment_success",name:"Payment Success",description:"Payment processed successfully"},{id:"payment_fail_1",name:"Payment Fail 1",description:"First payment attempt failed"},{id:"retry_payment_1",name:"Retry Payment 1",description:"Retry payment (attempt 2)"},{id:"payment_fail_2",name:"Payment Fail 2",description:"Second payment attempt failed"},{id:"retry_payment_2",name:"Retry Payment 2",description:"Retry payment (attempt 3)"},{id:"payment_fail_3",name:"Payment Fail 3",description:"Third payment attempt failed"},{id:"cancel_order",name:"Cancel Order",description:"Cancel order after max retries"},{id:"fulfill",name:"Fulfill",description:"Fulfill the order"}]},getPlaces(){return[{id:"cart",name:"Cart",initial:1},{id:"checkout_started",name:"CheckoutStarted",initial:0},{id:"payment_pending",name:"PaymentPending",initial:0},{id:"payment_processing",name:"PaymentProcessing",initial:0},{id:"retry_1",name:"Retry1",initial:0},{id:"retry_2",name:"Retry2",initial:0},{id:"retry_3",name:"Retry3",initial:0},{id:"paid",name:"Paid",initial:0},{id:"cancelled",name:"Cancelled",initial:0},{id:"fulfilled",name:"Fulfilled",initial:0}]},getTransition(e){return this.getTransitions().find(t=>t.id===e)||null},canFire(e){if(!this.getTransition(e))return{canFire:!1,reason:`Unknown transition: ${e}`};if(!o)return{canFire:!1,reason:"No current instance"};if(!this.isEnabled(e)){const a=this.getStatus();return{canFire:!1,reason:`Transition '${e}' not enabled in state '${a}'`,currentState:a,enabledTransitions:this.getEnabled()}}return{canFire:!0}},async sequence(e,t={}){const n=[],{stopOnError:a=!0,data:s={}}=t;for(const i of e){const c=this.canFire(i);if(!c.canFire){if(a)throw new Error(`Sequence failed at '${i}': ${c.reason}`);n.push({transition:i,success:!1,error:c.reason});continue}try{const r=await this.action(i,s[i]||{});n.push({transition:i,success:!0,state:r.state})}catch(r){if(a)throw r;n.push({transition:i,success:!1,error:r.message})}}return n},getWorkflowInfo(){var e;return{places:this.getPlaces(),transitions:this.getTransitions(),initialPlace:(e=this.getPlaces().find(t=>t.initial>0))==null?void 0:e.id}}};ve();Y();
