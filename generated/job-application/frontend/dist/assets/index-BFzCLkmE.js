(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const i of document.querySelectorAll('link[rel="modulepreload"]'))a(i);new MutationObserver(i=>{for(const o of i)if(o.type==="childList")for(const g of o.addedNodes)g.tagName==="LINK"&&g.rel==="modulepreload"&&a(g)}).observe(document,{childList:!0,subtree:!0});function n(i){const o={};return i.integrity&&(o.integrity=i.integrity),i.referrerPolicy&&(o.referrerPolicy=i.referrerPolicy),i.crossOrigin==="use-credentials"?o.credentials="include":i.crossOrigin==="anonymous"?o.credentials="omit":o.credentials="same-origin",o}function a(i){if(i.ep)return;i.ep=!0;const o=n(i);fetch(i.href,o)}})();const E=[{path:"/",component:"List",title:"job-application"},{path:"/job-application",component:"List",title:"job-application"},{path:"/job-application/new",component:"Form",title:"New job-application"},{path:"/job-application/:id",component:"Detail",title:"job-application Detail"},{path:"/admin",component:"AdminDashboard",title:"Admin Dashboard"},{path:"/admin/instances",component:"AdminInstances",title:"Instances"},{path:"/admin/instances/:id",component:"AdminInstance",title:"Instance Detail"}];let u=null,p={};function $(e){e=e||"/",e!=="/"&&e.endsWith("/")&&(e=e.slice(0,-1));for(const t of E){const n={};let a=t.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");a=a.replace(/:[^/]+/g,"([^/]+)");const i=new RegExp(`^${a}$`),o=e.match(i);if(o)return(t.path.match(/:[^/]+/g)||[]).map(j=>j.slice(1)).forEach((j,F)=>{n[j]=decodeURIComponent(o[F+1])}),{route:t,params:n}}return null}function m(e,t={}){e.startsWith("/")||(e="/"+e);const n=$(e);if(!n){console.warn(`No route found for path: ${e}, falling back to list`),e="/job-application";const a=$(e);a&&(u=a.route,p=a.params,window.history.pushState(t,"",e),L());return}if(n.route.roles&&n.route.roles.length>0){const a=U();if(!a||!q(a,n.route.roles)){console.warn("Access denied:",e),m("/job-application");return}}u=n.route,p=n.params,window.history.pushState(t,"",e),L()}window.addEventListener("popstate",()=>{const e=window.location.pathname,t=$(e);t?(u=t.route,p=t.params,L()):m("/job-application")});function U(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function q(e,t){return!e||!e.roles?!1:t.some(n=>e.roles.includes(n))}function L(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:u,params:p}}))}function z(){return p}function P(){return u}function V(){const e=window.location.pathname,t=$(e);t?(u=t.route,p=t.params):(u=E.find(n=>n.path==="/job-application")||E[0],p={})}const S={brand:"job-application",items:[{label:"job-application",path:"/job-application",icon:""},{label:"New",path:"/job-application/new",icon:"+"},{label:"Admin",path:"/admin",icon:""}]};let s=null,I=!1;async function x(){if(!I){I=!0;try{const e={},t=R();t&&(e.Authorization=`Bearer ${t}`);const n=await fetch("/api/navigation",{headers:e});n.ok?s=await n.json():s=S}catch{s=S}finally{I=!1}}}async function D(){s||await x();const e=window.location.pathname,t=K(),n=(s==null?void 0:s.items)||S.items;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="/job-application" onclick="handleNavClick(event, '/job-application')">
          ${(s==null?void 0:s.brand)||S.brand}
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
          <span class="user-name">${t.login||t.name||"User"}</span>
          <button onclick="handleLogout()" class="btn btn-link" style="color: rgba(255,255,255,0.8);">Logout</button>
        `:`
          <a href="/auth/login" class="btn btn-primary btn-sm">Login</a>
        `}
      </div>
    </nav>
  `}window.handleNavClick=function(e,t){e.preventDefault(),m(t)};window.handleLogout=async function(){try{const e=R();e&&await fetch("/auth/logout",{method:"POST",headers:{Authorization:`Bearer ${e}`}})}catch(e){console.error("Logout error:",e)}localStorage.removeItem("auth"),s=null,window.dispatchEvent(new CustomEvent("auth-change")),await N(),m("/job-application")};function K(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function R(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).token}catch{return null}return null}async function N(){s=null,await x();const e=document.getElementById("nav");e&&(e.innerHTML=await D())}window.addEventListener("auth-change",async()=>{await N()});window.addEventListener("route-change",()=>{const e=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(t=>{t.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(t=>{const n=t.getAttribute("href");(n===e||n!=="/"&&e.startsWith(n))&&t.parentElement.classList.add("active")})});let O=[];async function G(){try{const e=await fetch("/api/views");return e.ok?(O=await e.json(),O):(console.warn("Failed to load view definitions, using defaults"),[])}catch(e){return console.error("Error loading views:",e),[]}}const d="";let A=null,h=null,v=[],r=null;function Q(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);if(t.expires_at&&new Date(t.expires_at)>new Date)return h=t.token,A=t.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function M(e){localStorage.setItem("auth",JSON.stringify(e)),h=e.token,A=e.user,window.dispatchEvent(new CustomEvent("auth-change"))}function k(){localStorage.removeItem("auth"),h=null,A=null,window.dispatchEvent(new CustomEvent("auth-change"))}function l(){const e={"Content-Type":"application/json"};return h&&(e.Authorization=`Bearer ${h}`),e}async function w(e){if(e.status===401)throw k(),b("Session expired. Please log in again."),new Error("Unauthorized");if(!e.ok){const t=await e.json().catch(()=>({}));throw new Error(t.message||e.statusText)}return e.json()}const f={async getMe(){const e=await fetch(`${d}/auth/me`,{headers:l()});return w(e)},async logout(){await fetch(`${d}/auth/logout`,{method:"POST",headers:l()}),k()},async listInstances(){const e=await fetch(`${d}/admin/instances`,{headers:l()});return w(e)},async getInstance(e){const t=await fetch(`${d}/api/jobapplication/${e}`,{headers:l()});return w(t)},async createInstance(e={}){const t=await fetch(`${d}/api/jobapplication`,{method:"POST",headers:l(),body:JSON.stringify(e)});return w(t)},async executeTransition(e,t,n={}){const a=await fetch(`${d}/api/${e}`,{method:"POST",headers:l(),body:JSON.stringify({aggregate_id:t,data:n})});return w(a)}};window.api=f;window.setAuthToken=function(e){h=e};window.saveAuth=M;window.clearAuth=k;function b(e){const t=document.getElementById("app"),n=t.querySelector(".alert-error");n&&n.remove();const a=document.createElement("div");a.className="alert alert-error",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),5e3)}function H(e){const t=document.getElementById("app"),n=t.querySelector(".alert-success");n&&n.remove();const a=document.createElement("div");a.className="alert alert-success",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),3e3)}function T(e){if(!e)return"unknown";for(const[t,n]of Object.entries(e))if(n>0)return t;return"unknown"}function _(e){return`<span class="badge ${`badge-${e.toLowerCase().replace(/_/g,"-")}`}">${e.replace(/_/g," ")}</span>`}async function C(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>job-application</h1>
        <button class="btn btn-primary" onclick="handleCreateNew()">+ New</button>
      </div>
      <div id="instances-list" class="entity-list">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{v=(await f.listInstances()).instances||[],X()}catch{document.getElementById("instances-list").innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `}}function X(){const e=document.getElementById("instances-list");if(e){if(v.length===0){e.innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `;return}e.innerHTML=v.map(t=>{const n=T(t.state||t.places);return`
      <div class="entity-card" onclick="navigate('/job-application/${t.id}')">
        <div class="entity-info">
          <h3>${t.id}</h3>
          <div class="entity-meta">
            ${_(n)} &middot; Version ${t.version||0}
          </div>
        </div>
        <div class="entity-actions">
          <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); navigate('/job-application/${t.id}')">
            View
          </button>
        </div>
      </div>
    `}).join("")}}async function Y(){const t=z().id,n=document.getElementById("app");n.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/job-application')" style="margin-left: -0.5rem">
            &larr; Back to List
          </button>
          <h1 style="margin-top: 0.5rem">Instance: ${t}</h1>
        </div>
      </div>
      <div id="instance-detail">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{const a=await f.getInstance(t);r={id:a.aggregate_id||t,version:a.version,state:a.state,places:a.places,enabled:a.enabled||a.enabled_transitions||[]},J()}catch(a){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${a.message}</div>
    `}}function J(){const e=document.getElementById("instance-detail");if(!e||!r)return;const t=T(r.places),n=r.enabled||[],a=[{id:"start_screening",name:"Start Screening",description:"Begin candidate screening"},{id:"schedule_phone_screen",name:"Schedule Phone Screen",description:"Schedule phone screen"},{id:"start_background_check",name:"Start Background Check",description:"Initiate background check"},{id:"complete_phone_screen",name:"Complete Phone Screen",description:"Complete phone screen"},{id:"complete_background_check",name:"Complete Background Check",description:"Complete background check"},{id:"advance_to_interview",name:"Advance To Interview",description:"Both checks passed, advance to interview"},{id:"conduct_interview",name:"Conduct Interview",description:"Conduct interview"},{id:"extend_offer",name:"Extend Offer",description:"Extend job offer"},{id:"accept_offer",name:"Accept Offer",description:"Candidate accepts offer"},{id:"reject_after_screen",name:"Reject After Screen",description:"Reject after screening"},{id:"reject_after_interview",name:"Reject After Interview",description:"Reject after interview"},{id:"decline_offer",name:"Decline Offer",description:"Candidate declines offer"}];e.innerHTML=`
    <div class="card">
      <div class="card-header">Status</div>
      <div class="detail-list">
        <div class="detail-field">
          <dt>ID</dt>
          <dd><code>${r.id}</code></dd>
        </div>
        <div class="detail-field">
          <dt>Status</dt>
          <dd>${_(t)}</dd>
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
        ${Object.entries(r.places||{}).map(([i,o])=>`
          <div class="detail-field">
            <dt>${i}</dt>
            <dd>${o>0?`<span class="badge badge-${i}">${o} token${o>1?"s":""}</span>`:'<span style="color: #999;">0</span>'}</dd>
          </div>
        `).join("")}
      </div>
    </div>
  `}async function Z(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/job-application')" style="margin-left: -0.5rem">
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
            <button type="button" class="btn btn-secondary" onclick="navigate('/job-application')">Cancel</button>
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
  `;try{const[t,n]=await Promise.all([fetch(`${d}/admin/stats`,{headers:l()}).then(i=>i.json()).catch(()=>null),f.listInstances()]);t?document.getElementById("admin-stats").innerHTML=`
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
              ${v.slice(0,20).map(i=>{const o=T(i.state||i.places);return`
                  <tr>
                    <td><code>${i.id}</code></td>
                    <td>${_(o)}</td>
                    <td>${i.version||0}</td>
                    <td><button class="btn btn-sm btn-link" onclick="navigate('/job-application/${i.id}')">View</button></td>
                  </tr>
                `}).join("")}
            </tbody>
          </table>`:'<p style="color: #666; padding: 1rem;">No instances yet.</p>')}catch(t){b("Failed to load admin data: "+t.message)}}window.navigate=m;window.handleCreateNew=async function(){m("/job-application/new")};window.handleSubmitCreate=async function(e){e.preventDefault();try{const t=await f.createInstance({});H("Instance created successfully!"),m(`/job-application/${t.aggregate_id||t.id}`)}catch(t){b("Failed to create: "+t.message)}};window.handleTransition=async function(e){if(r)try{const t=await f.executeTransition(e,r.id);r={...r,version:t.version,state:t.state,places:t.state,enabled:t.enabled||[]},J(),H(`Action "${e}" completed!`)}catch(t){b(`Failed to execute ${e}: ${t.message}`)}};function B(e){var a;const t=((a=e.detail)==null?void 0:a.route)||P();if(!t){C();return}const n=t.path;n==="/job-application"||n==="/"?C():n==="/job-application/new"?Z():n==="/job-application/:id"?Y():n==="/admin"||n.startsWith("/admin")?ee():C()}async function te(){const e=new URLSearchParams(window.location.search),t=e.get("token"),n=e.get("expires_at");if(t){h=t;try{const a=await f.getMe();M({token:t,expires_at:n,user:a}),window.history.replaceState({},"",window.location.pathname),await N()}catch{k(),b("Failed to complete login")}}}async function ne(){Q(),await te(),await G();const e=document.getElementById("nav");e.innerHTML=await D(),window.addEventListener("route-change",B),V(),B({detail:{route:P()}})}let c=null,y=null;function W(){const t=`${window.location.protocol==="https:"?"wss:":"ws:"}//${window.location.host}/ws`;c=new WebSocket(t),c.onopen=()=>{console.log("[Debug] WebSocket connected")},c.onmessage=n=>{try{const a=JSON.parse(n.data);a.id==="session"&&a.type==="session"?(y=(typeof a.data=="string"?JSON.parse(a.data):a.data).session_id,console.log("[Debug] Session ID:",y)):a.type==="eval"&&ae(a)}catch(a){console.error("[Debug] Failed to parse message:",a)}},c.onclose=()=>{console.log("[Debug] WebSocket disconnected, reconnecting in 3s..."),y=null,setTimeout(W,3e3)},c.onerror=n=>{console.error("[Debug] WebSocket error:",n)}}async function ae(e){try{const n=(typeof e.data=="string"?JSON.parse(e.data):e.data).code,i=await new Function("return (async () => { "+n+" })()")(),o={type:"response",id:e.id,data:{result:i,type:typeof i}};c.send(JSON.stringify(o))}catch(t){const n={type:"response",id:e.id,data:{error:t.message}};c.send(JSON.stringify(n))}}window.debugSessionId=()=>y;window.debugWs=()=>c;ne();W();
