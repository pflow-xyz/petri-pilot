(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const s of document.querySelectorAll('link[rel="modulepreload"]'))a(s);new MutationObserver(s=>{for(const i of s)if(i.type==="childList")for(const v of i.addedNodes)v.tagName==="LINK"&&v.rel==="modulepreload"&&a(v)}).observe(document,{childList:!0,subtree:!0});function n(s){const i={};return s.integrity&&(i.integrity=s.integrity),s.referrerPolicy&&(i.referrerPolicy=s.referrerPolicy),s.crossOrigin==="use-credentials"?i.credentials="include":s.crossOrigin==="anonymous"?i.credentials="omit":i.credentials="same-origin",i}function a(s){if(s.ep)return;s.ep=!0;const i=n(s);fetch(s.href,i)}})();const N=[{path:"/",component:"List",title:"task-manager"},{path:"/task-manager",component:"List",title:"task-manager"},{path:"/task-manager/new",component:"Form",title:"New task-manager"},{path:"/task-manager/:id",component:"Detail",title:"task-manager Detail"},{path:"/admin",component:"AdminDashboard",title:"Admin Dashboard"},{path:"/admin/instances",component:"AdminInstances",title:"Instances"},{path:"/admin/instances/:id",component:"AdminInstance",title:"Instance Detail"}];let u=null,m={};function k(e){e=e||"/",e!=="/"&&e.endsWith("/")&&(e=e.slice(0,-1));for(const t of N){const n={};let a=t.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");a=a.replace(/:[^/]+/g,"([^/]+)");const s=new RegExp(`^${a}$`),i=e.match(s);if(i)return(t.path.match(/:[^/]+/g)||[]).map(I=>I.slice(1)).forEach((I,F)=>{n[I]=decodeURIComponent(i[F+1])}),{route:t,params:n}}return null}function h(e,t={}){e.startsWith("/")||(e="/"+e);const n=k(e);if(!n){console.warn(`No route found for path: ${e}, falling back to list`),e="/task-manager";const a=k(e);a&&(u=a.route,m=a.params,window.history.pushState(t,"",e),C());return}if(n.route.roles&&n.route.roles.length>0){const a=U();if(!a||!q(a,n.route.roles)){console.warn("Access denied:",e),h("/task-manager");return}}u=n.route,m=n.params,window.history.pushState(t,"",e),C()}window.addEventListener("popstate",()=>{const e=window.location.pathname,t=k(e);t?(u=t.route,m=t.params,C()):h("/task-manager")});function U(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function q(e,t){return!e||!e.roles?!1:t.some(n=>e.roles.includes(n))}function C(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:u,params:m}}))}function z(){return m}function x(){return u}function V(){const e=window.location.pathname,t=k(e);t?(u=t.route,m=t.params):(u=N.find(n=>n.path==="/task-manager")||N[0],m={})}const $={brand:"task-manager",items:[{label:"task-manager",path:"/task-manager",icon:""},{label:"New",path:"/task-manager/new",icon:"+"},{label:"Admin",path:"/admin",icon:""}]};let o=null,E=!1;async function M(){if(!E){E=!0;try{const e={},t=R();t&&(e.Authorization=`Bearer ${t}`);const n=await fetch("/api/navigation",{headers:e});n.ok?o=await n.json():o=$}catch{o=$}finally{E=!1}}}async function j(){o||await M();const e=window.location.pathname,t=K(),n=(o==null?void 0:o.items)||$.items;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="/task-manager" onclick="handleNavClick(event, '/task-manager')">
          ${(o==null?void 0:o.brand)||$.brand}
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
          <span class="user-name">${t.login||t.name||"User"}</span>
          <button onclick="handleLogout()" class="btn btn-link" style="color: rgba(255,255,255,0.8);">Logout</button>
        `:`
          <a href="/auth/login" class="btn btn-primary btn-sm">Login</a>
        `}
      </div>
    </nav>
  `}window.handleNavClick=function(e,t){e.preventDefault(),h(t)};window.handleLogout=async function(){try{const e=R();e&&await fetch("/auth/logout",{method:"POST",headers:{Authorization:`Bearer ${e}`}})}catch(e){console.error("Logout error:",e)}localStorage.removeItem("auth"),o=null,window.dispatchEvent(new CustomEvent("auth-change")),await A(),h("/task-manager")};function K(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function R(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).token}catch{return null}return null}async function A(){o=null,await M();const e=document.getElementById("nav");e&&(e.innerHTML=await j())}window.addEventListener("auth-change",async()=>{await A()});window.addEventListener("route-change",()=>{const e=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(t=>{t.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(t=>{const n=t.getAttribute("href");(n===e||n!=="/"&&e.startsWith(n))&&t.parentElement.classList.add("active")})});let B=[];async function G(){try{const e=await fetch("/api/views");return e.ok?(B=await e.json(),B):(console.warn("Failed to load view definitions, using defaults"),[])}catch(e){return console.error("Error loading views:",e),[]}}const d="";let T=null,g=null,f=[],r=null;function Q(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);if(t.expires_at&&new Date(t.expires_at)>new Date)return g=t.token,T=t.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function H(e){localStorage.setItem("auth",JSON.stringify(e)),g=e.token,T=e.user,window.dispatchEvent(new CustomEvent("auth-change"))}function S(){localStorage.removeItem("auth"),g=null,T=null,window.dispatchEvent(new CustomEvent("auth-change"))}function l(){const e={"Content-Type":"application/json"};return g&&(e.Authorization=`Bearer ${g}`),e}async function w(e){if(e.status===401)throw S(),y("Session expired. Please log in again."),new Error("Unauthorized");if(!e.ok){const t=await e.json().catch(()=>({}));throw new Error(t.message||e.statusText)}return e.json()}const p={async getMe(){const e=await fetch(`${d}/auth/me`,{headers:l()});return w(e)},async logout(){await fetch(`${d}/auth/logout`,{method:"POST",headers:l()}),S()},async listInstances(){const e=await fetch(`${d}/admin/instances`,{headers:l()});return w(e)},async getInstance(e){const t=await fetch(`${d}/api/taskmanager/${e}`,{headers:l()});return w(t)},async createInstance(e={}){const t=await fetch(`${d}/api/taskmanager`,{method:"POST",headers:l(),body:JSON.stringify(e)});return w(t)},async executeTransition(e,t,n={}){const a=await fetch(`${d}/api/${e}`,{method:"POST",headers:l(),body:JSON.stringify({aggregate_id:t,data:n})});return w(a)}};window.api=p;window.setAuthToken=function(e){g=e};window.saveAuth=H;window.clearAuth=S;function y(e){const t=document.getElementById("app"),n=t.querySelector(".alert-error");n&&n.remove();const a=document.createElement("div");a.className="alert alert-error",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),5e3)}function _(e){const t=document.getElementById("app"),n=t.querySelector(".alert-success");n&&n.remove();const a=document.createElement("div");a.className="alert alert-success",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),3e3)}function O(e){if(!e)return"unknown";for(const[t,n]of Object.entries(e))if(n>0)return t;return"unknown"}function P(e){return`<span class="badge ${`badge-${e.toLowerCase().replace(/_/g,"-")}`}">${e.replace(/_/g," ")}</span>`}async function L(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>task-manager</h1>
        <button class="btn btn-primary" onclick="handleCreateNew()">+ New</button>
      </div>
      <div id="instances-list" class="entity-list">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{f=(await p.listInstances()).instances||[],X()}catch{document.getElementById("instances-list").innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `}}function X(){const e=document.getElementById("instances-list");if(e){if(f.length===0){e.innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `;return}e.innerHTML=f.map(t=>{const n=O(t.state||t.places);return`
      <div class="entity-card" onclick="navigate('/task-manager/${t.id}')">
        <div class="entity-info">
          <h3>${t.id}</h3>
          <div class="entity-meta">
            ${P(n)} &middot; Version ${t.version||0}
          </div>
        </div>
        <div class="entity-actions">
          <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); navigate('/task-manager/${t.id}')">
            View
          </button>
        </div>
      </div>
    `}).join("")}}async function Y(){const t=z().id,n=document.getElementById("app");n.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/task-manager')" style="margin-left: -0.5rem">
            &larr; Back to List
          </button>
          <h1 style="margin-top: 0.5rem">Instance: ${t}</h1>
        </div>
      </div>
      <div id="instance-detail">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{const a=await p.getInstance(t);r={id:a.aggregate_id||t,version:a.version,state:a.state,places:a.places,enabled:a.enabled||a.enabled_transitions||[]},J()}catch(a){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${a.message}</div>
    `}}function J(){const e=document.getElementById("instance-detail");if(!e||!r)return;const t=O(r.places),n=r.enabled||[],a=[{id:"start",name:"Start",description:"Start working on a task"},{id:"submit",name:"Submit",description:"Submit task for review"},{id:"approve",name:"Approve",description:"Approve completed task"},{id:"reject",name:"Reject",description:"Reject task and send back"}];e.innerHTML=`
    <div class="card">
      <div class="card-header">Status</div>
      <div class="detail-list">
        <div class="detail-field">
          <dt>ID</dt>
          <dd><code>${r.id}</code></dd>
        </div>
        <div class="detail-field">
          <dt>Status</dt>
          <dd>${P(t)}</dd>
        </div>
        <div class="detail-field">
          <dt>Version</dt>
          <dd>${r.version||0}</dd>
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
        ${Object.entries(r.places||{}).map(([s,i])=>`
          <div class="detail-field">
            <dt>${s}</dt>
            <dd>${i>0?`<span class="badge badge-${s}">${i} token${i>1?"s":""}</span>`:'<span style="color: #999;">0</span>'}</dd>
          </div>
        `).join("")}
      </div>
    </div>
  `}async function Z(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/task-manager')" style="margin-left: -0.5rem">
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
            <button type="button" class="btn btn-secondary" onclick="navigate('/task-manager')">Cancel</button>
          </div>
        </form>
      </div>
    </div>
  `}async function ee(){const e=document.getElementById("app");e.innerHTML=`
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
  `;try{const[t,n]=await Promise.all([fetch(`${d}/admin/stats`,{headers:l()}).then(s=>s.json()).catch(()=>null),p.listInstances()]);t?document.getElementById("admin-stats").innerHTML=`
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
      `:document.getElementById("admin-stats").innerHTML="",f=n.instances||[];const a=document.getElementById("admin-instances").querySelector(".loading");a&&(a.outerHTML=f.length>0?`<table class="table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Status</th>
                <th>Version</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              ${f.slice(0,20).map(s=>{const i=O(s.state||s.places);return`
                  <tr>
                    <td><code>${s.id}</code></td>
                    <td>${P(i)}</td>
                    <td>${s.version||0}</td>
                    <td><button class="btn btn-sm btn-link" onclick="navigate('/task-manager/${s.id}')">View</button></td>
                  </tr>
                `}).join("")}
            </tbody>
          </table>`:'<p style="color: #666; padding: 1rem;">No instances yet.</p>')}catch(t){y("Failed to load admin data: "+t.message)}}window.navigate=h;window.handleCreateNew=async function(){h("/task-manager/new")};window.handleSubmitCreate=async function(e){e.preventDefault();try{const t=await p.createInstance({});_("Instance created successfully!"),h(`/task-manager/${t.aggregate_id||t.id}`)}catch(t){y("Failed to create: "+t.message)}};window.handleTransition=async function(e){if(r)try{const t=await p.executeTransition(e,r.id);r={...r,version:t.version,state:t.state,places:t.state,enabled:t.enabled||[]},J(),_(`Action "${e}" completed!`)}catch(t){y(`Failed to execute ${e}: ${t.message}`)}};function D(e){var a;const t=((a=e.detail)==null?void 0:a.route)||x();if(!t){L();return}const n=t.path;n==="/task-manager"||n==="/"?L():n==="/task-manager/new"?Z():n==="/task-manager/:id"?Y():n==="/admin"||n.startsWith("/admin")?ee():L()}async function te(){const e=new URLSearchParams(window.location.search),t=e.get("token"),n=e.get("expires_at");if(t){g=t;try{const a=await p.getMe();H({token:t,expires_at:n,user:a}),window.history.replaceState({},"",window.location.pathname),await A()}catch{S(),y("Failed to complete login")}}}async function ne(){Q(),await te(),await G();const e=document.getElementById("nav");e.innerHTML=await j(),window.addEventListener("route-change",D),V(),D({detail:{route:x()}})}let c=null,b=null;function W(){const t=`${window.location.protocol==="https:"?"wss:":"ws:"}//${window.location.host}/ws`;c=new WebSocket(t),c.onopen=()=>{console.log("[Debug] WebSocket connected")},c.onmessage=n=>{try{const a=JSON.parse(n.data);a.id==="session"&&a.type==="session"?(b=(typeof a.data=="string"?JSON.parse(a.data):a.data).session_id,console.log("[Debug] Session ID:",b)):a.type==="eval"&&ae(a)}catch(a){console.error("[Debug] Failed to parse message:",a)}},c.onclose=()=>{console.log("[Debug] WebSocket disconnected, reconnecting in 3s..."),b=null,setTimeout(W,3e3)},c.onerror=n=>{console.error("[Debug] WebSocket error:",n)}}async function ae(e){try{const n=(typeof e.data=="string"?JSON.parse(e.data):e.data).code,s=await new Function("return (async () => { "+n+" })()")(),i={type:"response",id:e.id,data:{result:s,type:typeof s}};c.send(JSON.stringify(i))}catch(t){const n={type:"response",id:e.id,data:{error:t.message}};c.send(JSON.stringify(n))}}window.debugSessionId=()=>b;window.debugWs=()=>c;ne();W();
