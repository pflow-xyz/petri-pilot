(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const s of document.querySelectorAll('link[rel="modulepreload"]'))a(s);new MutationObserver(s=>{for(const r of s)if(r.type==="childList")for(const v of r.addedNodes)v.tagName==="LINK"&&v.rel==="modulepreload"&&a(v)}).observe(document,{childList:!0,subtree:!0});function n(s){const r={};return s.integrity&&(r.integrity=s.integrity),s.referrerPolicy&&(r.referrerPolicy=s.referrerPolicy),s.crossOrigin==="use-credentials"?r.credentials="include":s.crossOrigin==="anonymous"?r.credentials="omit":r.credentials="same-origin",r}function a(s){if(s.ep)return;s.ep=!0;const r=n(s);fetch(s.href,r)}})();const E=[{path:"/",component:"List",title:"order-processing"},{path:"/order-processing",component:"List",title:"order-processing"},{path:"/order-processing/new",component:"Form",title:"New order-processing"},{path:"/order-processing/:id",component:"Detail",title:"order-processing Detail"},{path:"/admin",component:"AdminDashboard",title:"Admin Dashboard"},{path:"/admin/instances",component:"AdminInstances",title:"Instances"},{path:"/admin/instances/:id",component:"AdminInstance",title:"Instance Detail"}];let l=null,u={};function y(e){e=e||"/",e!=="/"&&e.endsWith("/")&&(e=e.slice(0,-1));for(const t of E){const n={};let a=t.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");a=a.replace(/:[^/]+/g,"([^/]+)");const s=new RegExp(`^${a}$`),r=e.match(s);if(r)return(t.path.match(/:[^/]+/g)||[]).map($=>$.slice(1)).forEach(($,_)=>{n[$]=decodeURIComponent(r[_+1])}),{route:t,params:n}}return null}function p(e,t={}){e.startsWith("/")||(e="/"+e);const n=y(e);if(!n){console.warn(`No route found for path: ${e}, falling back to list`),e="/order-processing";const a=y(e);a&&(l=a.route,u=a.params,window.history.pushState(t,"",e),L());return}if(n.route.roles&&n.route.roles.length>0){const a=D();if(!a||!F(a,n.route.roles)){console.warn("Access denied:",e),p("/order-processing");return}}l=n.route,u=n.params,window.history.pushState(t,"",e),L()}window.addEventListener("popstate",()=>{const e=window.location.pathname,t=y(e);t?(l=t.route,u=t.params,L()):p("/order-processing")});function D(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function F(e,t){return!e||!e.roles?!1:t.some(n=>e.roles.includes(n))}function L(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:l,params:u}}))}function J(){return u}function O(){return l}function U(){const e=window.location.pathname,t=y(e);t?(l=t.route,u=t.params):(l=E.find(n=>n.path==="/order-processing")||E[0],u={})}const b={brand:"order-processing",items:[{label:"order-processing",path:"/order-processing",icon:""},{label:"New",path:"/order-processing/new",icon:"+"},{label:"Admin",path:"/admin",icon:""}]};let i=null,I=!1;async function M(){if(!I){I=!0;try{const e={},t=j();t&&(e.Authorization=`Bearer ${t}`);const n=await fetch("/api/navigation",{headers:e});n.ok?i=await n.json():i=b}catch{i=b}finally{I=!1}}}async function x(){i||await M();const e=window.location.pathname,t=V(),n=(i==null?void 0:i.items)||b.items;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="/order-processing" onclick="handleNavClick(event, '/order-processing')">
          ${(i==null?void 0:i.brand)||b.brand}
        </a>
      </div>
      <ul class="nav-menu">
        ${n.map(r=>`
            <li class="${e===r.path||r.path!=="/"&&e.startsWith(r.path)?"active":""}">
              <a href="${r.path}" onclick="handleNavClick(event, '${r.path}')">
                ${r.icon?`<span class="icon">${r.icon}</span>`:""}
                ${r.label}
              </a>
            </li>
          `).join("")}
      </ul>
      <div class="nav-user">
        ${t?`
          <span class="user-name">${t.login||t.name||"User"}</span>
          <button onclick="handleLogout()" class="btn btn-link" style="color: rgba(255,255,255,0.8);">Logout</button>
        `:`
          <a href="/auth/login" class="btn btn-primary btn-sm">Login</a>
        `}
      </div>
    </nav>
  `}window.handleNavClick=function(e,t){e.preventDefault(),p(t)};window.handleLogout=async function(){try{const e=j();e&&await fetch("/auth/logout",{method:"POST",headers:{Authorization:`Bearer ${e}`}})}catch(e){console.error("Logout error:",e)}localStorage.removeItem("auth"),i=null,window.dispatchEvent(new CustomEvent("auth-change")),await C(),p("/order-processing")};function V(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function j(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).token}catch{return null}return null}async function C(){i=null,await M();const e=document.getElementById("nav");e&&(e.innerHTML=await x())}window.addEventListener("auth-change",async()=>{await C()});window.addEventListener("route-change",()=>{const e=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(t=>{t.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(t=>{const n=t.getAttribute("href");(n===e||n!=="/"&&e.startsWith(n))&&t.parentElement.classList.add("active")})});let P=[];async function q(){try{const e=await fetch("/api/views");return e.ok?(P=await e.json(),P):(console.warn("Failed to load view definitions, using defaults"),[])}catch(e){return console.error("Error loading views:",e),[]}}const c="";let k=null,g=null,h=[],o=null;function z(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);if(t.expires_at&&new Date(t.expires_at)>new Date)return g=t.token,k=t.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function W(e){localStorage.setItem("auth",JSON.stringify(e)),g=e.token,k=e.user,window.dispatchEvent(new CustomEvent("auth-change"))}function N(){localStorage.removeItem("auth"),g=null,k=null,window.dispatchEvent(new CustomEvent("auth-change"))}function d(){const e={"Content-Type":"application/json"};return g&&(e.Authorization=`Bearer ${g}`),e}async function f(e){if(e.status===401)throw N(),w("Session expired. Please log in again."),new Error("Unauthorized");if(!e.ok){const t=await e.json().catch(()=>({}));throw new Error(t.message||e.statusText)}return e.json()}const m={async getMe(){const e=await fetch(`${c}/auth/me`,{headers:d()});return f(e)},async logout(){await fetch(`${c}/auth/logout`,{method:"POST",headers:d()}),N()},async listInstances(){const e=await fetch(`${c}/admin/instances`,{headers:d()});return f(e)},async getInstance(e){const t=await fetch(`${c}/api/order-processing/${e}`,{headers:d()});return f(t)},async createInstance(e={}){const t=await fetch(`${c}/api/order-processing`,{method:"POST",headers:d(),body:JSON.stringify(e)});return f(t)},async executeTransition(e,t,n={}){const a=await fetch(`${c}/api/order-processing/transitions/${e}`,{method:"POST",headers:d(),body:JSON.stringify({aggregate_id:t,data:n})});return f(a)}};window.api=m;function w(e){const t=document.getElementById("app"),n=t.querySelector(".alert-error");n&&n.remove();const a=document.createElement("div");a.className="alert alert-error",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),5e3)}function H(e){const t=document.getElementById("app"),n=t.querySelector(".alert-success");n&&n.remove();const a=document.createElement("div");a.className="alert alert-success",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),3e3)}function T(e){if(!e)return"unknown";for(const[t,n]of Object.entries(e))if(n>0)return t;return"unknown"}function A(e){return`<span class="badge ${`badge-${e.toLowerCase().replace(/_/g,"-")}`}">${e.replace(/_/g," ")}</span>`}async function S(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>order-processing</h1>
        <button class="btn btn-primary" onclick="handleCreateNew()">+ New</button>
      </div>
      <div id="instances-list" class="entity-list">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{h=(await m.listInstances()).instances||[],K()}catch{document.getElementById("instances-list").innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `}}function K(){const e=document.getElementById("instances-list");if(e){if(h.length===0){e.innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `;return}e.innerHTML=h.map(t=>{const n=T(t.state||t.places);return`
      <div class="entity-card" onclick="navigate('/order-processing/${t.id}')">
        <div class="entity-info">
          <h3>${t.id}</h3>
          <div class="entity-meta">
            ${A(n)} &middot; Version ${t.version||0}
          </div>
        </div>
        <div class="entity-actions">
          <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); navigate('/order-processing/${t.id}')">
            View
          </button>
        </div>
      </div>
    `}).join("")}}async function G(){const t=J().id,n=document.getElementById("app");n.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/order-processing')" style="margin-left: -0.5rem">
            &larr; Back to List
          </button>
          <h1 style="margin-top: 0.5rem">Instance: ${t}</h1>
        </div>
      </div>
      <div id="instance-detail">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{const a=await m.getInstance(t);o={id:a.aggregate_id||t,version:a.version,state:a.state,places:a.places,enabled:a.enabled||a.enabled_transitions||[]},R()}catch(a){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${a.message}</div>
    `}}function R(){const e=document.getElementById("instance-detail");if(!e||!o)return;const t=T(o.places),n=o.enabled||[],a=[{id:"validate",name:"Validate",description:"Check order validity"},{id:"reject",name:"Reject",description:"Mark order as invalid"},{id:"process_payment",name:"Process Payment",description:"Charge customer payment"},{id:"ship",name:"Ship",description:"Send order to shipping"},{id:"confirm",name:"Confirm",description:"Mark order as complete"}];e.innerHTML=`
    <div class="card">
      <div class="card-header">Status</div>
      <div class="detail-list">
        <div class="detail-field">
          <dt>ID</dt>
          <dd><code>${o.id}</code></dd>
        </div>
        <div class="detail-field">
          <dt>Status</dt>
          <dd>${A(t)}</dd>
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
        ${a.map(s=>{const r=n.includes(s.id);return`
            <button
              class="btn ${r?"btn-primary":"btn-secondary"}"
              onclick="handleTransition('${s.id}')"
              ${r?"":"disabled"}
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
        ${Object.entries(o.places||{}).map(([s,r])=>`
          <div class="detail-field">
            <dt>${s}</dt>
            <dd>${r>0?`<span class="badge badge-${s}">${r} token${r>1?"s":""}</span>`:'<span style="color: #999;">0</span>'}</dd>
          </div>
        `).join("")}
      </div>
    </div>
  `}async function Q(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/order-processing')" style="margin-left: -0.5rem">
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
            <button type="button" class="btn btn-secondary" onclick="navigate('/order-processing')">Cancel</button>
          </div>
        </form>
      </div>
    </div>
  `}async function X(){const e=document.getElementById("app");e.innerHTML=`
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
  `;try{const[t,n]=await Promise.all([fetch(`${c}/admin/stats`,{headers:d()}).then(s=>s.json()).catch(()=>null),m.listInstances()]);t?document.getElementById("admin-stats").innerHTML=`
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
      `:document.getElementById("admin-stats").innerHTML="",h=n.instances||[];const a=document.getElementById("admin-instances").querySelector(".loading");a&&(a.outerHTML=h.length>0?`<table class="table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Status</th>
                <th>Version</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              ${h.slice(0,20).map(s=>{const r=T(s.state||s.places);return`
                  <tr>
                    <td><code>${s.id}</code></td>
                    <td>${A(r)}</td>
                    <td>${s.version||0}</td>
                    <td><button class="btn btn-sm btn-link" onclick="navigate('/order-processing/${s.id}')">View</button></td>
                  </tr>
                `}).join("")}
            </tbody>
          </table>`:'<p style="color: #666; padding: 1rem;">No instances yet.</p>')}catch(t){w("Failed to load admin data: "+t.message)}}window.navigate=p;window.handleCreateNew=async function(){p("/order-processing/new")};window.handleSubmitCreate=async function(e){e.preventDefault();try{const t=await m.createInstance({});H("Instance created successfully!"),p(`/order-processing/${t.aggregate_id||t.id}`)}catch(t){w("Failed to create: "+t.message)}};window.handleTransition=async function(e){if(o)try{const t=await m.executeTransition(e,o.id);o={...o,version:t.version,state:t.state,places:t.state,enabled:t.enabled||[]},R(),H(`Action "${e}" completed!`)}catch(t){w(`Failed to execute ${e}: ${t.message}`)}};function B(e){var a;const t=((a=e.detail)==null?void 0:a.route)||O();if(!t){S();return}const n=t.path;n==="/order-processing"||n==="/"?S():n==="/order-processing/new"?Q():n==="/order-processing/:id"?G():n==="/admin"||n.startsWith("/admin")?X():S()}async function Y(){const e=new URLSearchParams(window.location.search),t=e.get("token"),n=e.get("expires_at");if(t){g=t;try{const a=await m.getMe();W({token:t,expires_at:n,user:a}),window.history.replaceState({},"",window.location.pathname),await C()}catch{N(),w("Failed to complete login")}}}async function Z(){z(),await Y(),await q();const e=document.getElementById("nav");e.innerHTML=await x(),window.addEventListener("route-change",B),U(),B({detail:{route:O()}})}Z();
