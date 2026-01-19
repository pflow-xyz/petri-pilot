(function(){const e=document.createElement("link").relList;if(e&&e.supports&&e.supports("modulepreload"))return;for(const s of document.querySelectorAll('link[rel="modulepreload"]'))a(s);new MutationObserver(s=>{for(const i of s)if(i.type==="childList")for(const g of i.addedNodes)g.tagName==="LINK"&&g.rel==="modulepreload"&&a(g)}).observe(document,{childList:!0,subtree:!0});function n(s){const i={};return s.integrity&&(i.integrity=s.integrity),s.referrerPolicy&&(i.referrerPolicy=s.referrerPolicy),s.crossOrigin==="use-credentials"?i.credentials="include":s.crossOrigin==="anonymous"?i.credentials="omit":i.credentials="same-origin",i}function a(s){if(s.ep)return;s.ep=!0;const i=n(s);fetch(s.href,i)}})();const L=[{path:"/",component:"List",title:"support-ticket"},{path:"/support-ticket",component:"List",title:"support-ticket"},{path:"/support-ticket/new",component:"Form",title:"New support-ticket"},{path:"/support-ticket/:id",component:"Detail",title:"support-ticket Detail"},{path:"/admin",component:"AdminDashboard",title:"Admin Dashboard"},{path:"/admin/instances",component:"AdminInstances",title:"Instances"},{path:"/admin/instances/:id",component:"AdminInstance",title:"Instance Detail"}];let u=null,p={};function k(t){t=t||"/",t!=="/"&&t.endsWith("/")&&(t=t.slice(0,-1));for(const e of L){const n={};let a=e.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");a=a.replace(/:[^/]+/g,"([^/]+)");const s=new RegExp(`^${a}$`),i=t.match(s);if(i)return(e.path.match(/:[^/]+/g)||[]).map(I=>I.slice(1)).forEach((I,F)=>{n[I]=decodeURIComponent(i[F+1])}),{route:e,params:n}}return null}function h(t,e={}){t.startsWith("/")||(t="/"+t);const n=k(t);if(!n){console.warn(`No route found for path: ${t}, falling back to list`),t="/support-ticket";const a=k(t);a&&(u=a.route,p=a.params,window.history.pushState(e,"",t),N());return}if(n.route.roles&&n.route.roles.length>0){const a=q();if(!a||!U(a,n.route.roles)){console.warn("Access denied:",t),h("/support-ticket");return}}u=n.route,p=n.params,window.history.pushState(e,"",t),N()}window.addEventListener("popstate",()=>{const t=window.location.pathname,e=k(t);e?(u=e.route,p=e.params,N()):h("/support-ticket")});function q(){const t=localStorage.getItem("auth");if(t)try{return JSON.parse(t).user}catch{return null}return null}function U(t,e){return!t||!t.roles?!1:e.some(n=>t.roles.includes(n))}function N(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:u,params:p}}))}function z(){return p}function R(){return u}function V(){const t=window.location.pathname,e=k(t);e?(u=e.route,p=e.params):(u=L.find(n=>n.path==="/support-ticket")||L[0],p={})}const $={brand:"support-ticket",items:[{label:"support-ticket",path:"/support-ticket",icon:""},{label:"New",path:"/support-ticket/new",icon:"+"},{label:"Admin",path:"/admin",icon:""}]};let o=null,E=!1;async function x(){if(!E){E=!0;try{const t={},e=_();e&&(t.Authorization=`Bearer ${e}`);const n=await fetch("/api/navigation",{headers:t});n.ok?o=await n.json():o=$}catch{o=$}finally{E=!1}}}async function M(){o||await x();const t=window.location.pathname,e=K(),n=(o==null?void 0:o.items)||$.items;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="/support-ticket" onclick="handleNavClick(event, '/support-ticket')">
          ${(o==null?void 0:o.brand)||$.brand}
        </a>
      </div>
      <ul class="nav-menu">
        ${n.map(i=>`
            <li class="${t===i.path||i.path!=="/"&&t.startsWith(i.path)?"active":""}">
              <a href="${i.path}" onclick="handleNavClick(event, '${i.path}')">
                ${i.icon?`<span class="icon">${i.icon}</span>`:""}
                ${i.label}
              </a>
            </li>
          `).join("")}
      </ul>
      <div class="nav-user">
        ${e?`
          <span class="user-name">${e.login||e.name||"User"}</span>
          <button onclick="handleLogout()" class="btn btn-link" style="color: rgba(255,255,255,0.8);">Logout</button>
        `:`
          <a href="/auth/login" class="btn btn-primary btn-sm">Login</a>
        `}
      </div>
    </nav>
  `}window.handleNavClick=function(t,e){t.preventDefault(),h(e)};window.handleLogout=async function(){try{const t=_();t&&await fetch("/auth/logout",{method:"POST",headers:{Authorization:`Bearer ${t}`}})}catch(t){console.error("Logout error:",t)}localStorage.removeItem("auth"),o=null,window.dispatchEvent(new CustomEvent("auth-change")),await A(),h("/support-ticket")};function K(){const t=localStorage.getItem("auth");if(t)try{return JSON.parse(t).user}catch{return null}return null}function _(){const t=localStorage.getItem("auth");if(t)try{return JSON.parse(t).token}catch{return null}return null}async function A(){o=null,await x();const t=document.getElementById("nav");t&&(t.innerHTML=await M())}window.addEventListener("auth-change",async()=>{await A()});window.addEventListener("route-change",()=>{const t=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(e=>{e.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(e=>{const n=e.getAttribute("href");(n===t||n!=="/"&&t.startsWith(n))&&e.parentElement.classList.add("active")})});let P=[];async function G(){try{const t=await fetch("/api/views");return t.ok?(P=await t.json(),P):(console.warn("Failed to load view definitions, using defaults"),[])}catch(t){return console.error("Error loading views:",t),[]}}const d="";let T=null,m=null,v=[],r=null;function Q(){const t=localStorage.getItem("auth");if(t)try{const e=JSON.parse(t);if(e.expires_at&&new Date(e.expires_at)>new Date)return m=e.token,T=e.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function H(t){localStorage.setItem("auth",JSON.stringify(t)),m=t.token,T=t.user,window.dispatchEvent(new CustomEvent("auth-change"))}function S(){localStorage.removeItem("auth"),m=null,T=null,window.dispatchEvent(new CustomEvent("auth-change"))}function l(){const t={"Content-Type":"application/json"};return m&&(t.Authorization=`Bearer ${m}`),t}async function w(t){if(t.status===401)throw S(),y("Session expired. Please log in again."),new Error("Unauthorized");if(!t.ok){const e=await t.json().catch(()=>({}));throw new Error(e.message||t.statusText)}return t.json()}const f={async getMe(){const t=await fetch(`${d}/auth/me`,{headers:l()});return w(t)},async logout(){await fetch(`${d}/auth/logout`,{method:"POST",headers:l()}),S()},async listInstances(){const t=await fetch(`${d}/admin/instances`,{headers:l()});return w(t)},async getInstance(t){const e=await fetch(`${d}/api/supportticket/${t}`,{headers:l()});return w(e)},async createInstance(t={}){const e=await fetch(`${d}/api/supportticket`,{method:"POST",headers:l(),body:JSON.stringify(t)});return w(e)},async executeTransition(t,e,n={}){const a=await fetch(`${d}/api/${t}`,{method:"POST",headers:l(),body:JSON.stringify({aggregate_id:e,data:n})});return w(a)}};window.api=f;window.setAuthToken=function(t){m=t};window.saveAuth=H;window.clearAuth=S;function y(t){const e=document.getElementById("app"),n=e.querySelector(".alert-error");n&&n.remove();const a=document.createElement("div");a.className="alert alert-error",a.textContent=t,e.insertBefore(a,e.firstChild),setTimeout(()=>a.remove(),5e3)}function j(t){const e=document.getElementById("app"),n=e.querySelector(".alert-success");n&&n.remove();const a=document.createElement("div");a.className="alert alert-success",a.textContent=t,e.insertBefore(a,e.firstChild),setTimeout(()=>a.remove(),3e3)}function O(t){if(!t)return"unknown";for(const[e,n]of Object.entries(t))if(n>0)return e;return"unknown"}function B(t){return`<span class="badge ${`badge-${t.toLowerCase().replace(/_/g,"-")}`}">${t.replace(/_/g," ")}</span>`}async function C(){const t=document.getElementById("app");t.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>support-ticket</h1>
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
    `}}function X(){const t=document.getElementById("instances-list");if(t){if(v.length===0){t.innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `;return}t.innerHTML=v.map(e=>{const n=O(e.state||e.places);return`
      <div class="entity-card" onclick="navigate('/support-ticket/${e.id}')">
        <div class="entity-info">
          <h3>${e.id}</h3>
          <div class="entity-meta">
            ${B(n)} &middot; Version ${e.version||0}
          </div>
        </div>
        <div class="entity-actions">
          <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); navigate('/support-ticket/${e.id}')">
            View
          </button>
        </div>
      </div>
    `}).join("")}}async function Y(){const e=z().id,n=document.getElementById("app");n.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/support-ticket')" style="margin-left: -0.5rem">
            &larr; Back to List
          </button>
          <h1 style="margin-top: 0.5rem">Instance: ${e}</h1>
        </div>
      </div>
      <div id="instance-detail">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{const a=await f.getInstance(e);r={id:a.aggregate_id||e,version:a.version,state:a.state,places:a.places,enabled:a.enabled||a.enabled_transitions||[]},W()}catch(a){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${a.message}</div>
    `}}function W(){const t=document.getElementById("instance-detail");if(!t||!r)return;const e=O(r.places),n=r.enabled||[],a=[{id:"assign",name:"Assign",description:"Assign ticket to an agent"},{id:"start_work",name:"Start Work",description:"Begin working on the ticket"},{id:"escalate",name:"Escalate",description:"Escalate to senior support"},{id:"request_info",name:"Request Info",description:"Request more information from customer"},{id:"customer_reply",name:"Customer Reply",description:"Customer provides requested information"},{id:"resolve",name:"Resolve",description:"Mark issue as resolved"},{id:"close",name:"Close",description:"Close the ticket"},{id:"reopen",name:"Reopen",description:"Customer reopens a closed ticket"}];t.innerHTML=`
    <div class="card">
      <div class="card-header">Status</div>
      <div class="detail-list">
        <div class="detail-field">
          <dt>ID</dt>
          <dd><code>${r.id}</code></dd>
        </div>
        <div class="detail-field">
          <dt>Status</dt>
          <dd>${B(e)}</dd>
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
  `}async function Z(){const t=document.getElementById("app");t.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/support-ticket')" style="margin-left: -0.5rem">
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
            <button type="button" class="btn btn-secondary" onclick="navigate('/support-ticket')">Cancel</button>
          </div>
        </form>
      </div>
    </div>
  `}async function tt(){const t=document.getElementById("app");t.innerHTML=`
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
  `;try{const[e,n]=await Promise.all([fetch(`${d}/admin/stats`,{headers:l()}).then(s=>s.json()).catch(()=>null),f.listInstances()]);e?document.getElementById("admin-stats").innerHTML=`
        <div class="card-header">Statistics</div>
        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 1rem;">
          <div>
            <div style="font-size: 2rem; font-weight: 600;">${e.total_streams||0}</div>
            <div style="color: #666;">Total Instances</div>
          </div>
          <div>
            <div style="font-size: 2rem; font-weight: 600;">${e.total_events||0}</div>
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
              ${v.slice(0,20).map(s=>{const i=O(s.state||s.places);return`
                  <tr>
                    <td><code>${s.id}</code></td>
                    <td>${B(i)}</td>
                    <td>${s.version||0}</td>
                    <td><button class="btn btn-sm btn-link" onclick="navigate('/support-ticket/${s.id}')">View</button></td>
                  </tr>
                `}).join("")}
            </tbody>
          </table>`:'<p style="color: #666; padding: 1rem;">No instances yet.</p>')}catch(e){y("Failed to load admin data: "+e.message)}}window.navigate=h;window.handleCreateNew=async function(){h("/support-ticket/new")};window.handleSubmitCreate=async function(t){t.preventDefault();try{const e=await f.createInstance({});j("Instance created successfully!"),h(`/support-ticket/${e.aggregate_id||e.id}`)}catch(e){y("Failed to create: "+e.message)}};window.handleTransition=async function(t){if(r)try{const e=await f.executeTransition(t,r.id);r={...r,version:e.version,state:e.state,places:e.state,enabled:e.enabled||[]},W(),j(`Action "${t}" completed!`)}catch(e){y(`Failed to execute ${t}: ${e.message}`)}};function D(t){var a;const e=((a=t.detail)==null?void 0:a.route)||R();if(!e){C();return}const n=e.path;n==="/support-ticket"||n==="/"?C():n==="/support-ticket/new"?Z():n==="/support-ticket/:id"?Y():n==="/admin"||n.startsWith("/admin")?tt():C()}async function et(){const t=new URLSearchParams(window.location.search),e=t.get("token"),n=t.get("expires_at");if(e){m=e;try{const a=await f.getMe();H({token:e,expires_at:n,user:a}),window.history.replaceState({},"",window.location.pathname),await A()}catch{S(),y("Failed to complete login")}}}async function nt(){Q(),await et(),await G();const t=document.getElementById("nav");t.innerHTML=await M(),window.addEventListener("route-change",D),V(),D({detail:{route:R()}})}let c=null,b=null;function J(){const e=`${window.location.protocol==="https:"?"wss:":"ws:"}//${window.location.host}/ws`;c=new WebSocket(e),c.onopen=()=>{console.log("[Debug] WebSocket connected")},c.onmessage=n=>{try{const a=JSON.parse(n.data);a.id==="session"&&a.type==="session"?(b=(typeof a.data=="string"?JSON.parse(a.data):a.data).session_id,console.log("[Debug] Session ID:",b)):a.type==="eval"&&at(a)}catch(a){console.error("[Debug] Failed to parse message:",a)}},c.onclose=()=>{console.log("[Debug] WebSocket disconnected, reconnecting in 3s..."),b=null,setTimeout(J,3e3)},c.onerror=n=>{console.error("[Debug] WebSocket error:",n)}}async function at(t){try{const n=(typeof t.data=="string"?JSON.parse(t.data):t.data).code,s=await new Function("return (async () => { "+n+" })()")(),i={type:"response",id:t.id,data:{result:s,type:typeof s}};c.send(JSON.stringify(i))}catch(e){const n={type:"response",id:t.id,data:{error:e.message}};c.send(JSON.stringify(n))}}window.debugSessionId=()=>b;window.debugWs=()=>c;nt();J();
