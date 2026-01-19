(function(){const e=document.createElement("link").relList;if(e&&e.supports&&e.supports("modulepreload"))return;for(const s of document.querySelectorAll('link[rel="modulepreload"]'))a(s);new MutationObserver(s=>{for(const o of s)if(o.type==="childList")for(const v of o.addedNodes)v.tagName==="LINK"&&v.rel==="modulepreload"&&a(v)}).observe(document,{childList:!0,subtree:!0});function n(s){const o={};return s.integrity&&(o.integrity=s.integrity),s.referrerPolicy&&(o.referrerPolicy=s.referrerPolicy),s.crossOrigin==="use-credentials"?o.credentials="include":s.crossOrigin==="anonymous"?o.credentials="omit":o.credentials="same-origin",o}function a(s){if(s.ep)return;s.ep=!0;const o=n(s);fetch(s.href,o)}})();const N=[{path:"/",component:"List",title:"blog-post"},{path:"/blog-post",component:"List",title:"blog-post"},{path:"/blog-post/new",component:"Form",title:"New blog-post"},{path:"/blog-post/:id",component:"Detail",title:"blog-post Detail"},{path:"/admin",component:"AdminDashboard",title:"Admin Dashboard"},{path:"/admin/instances",component:"AdminInstances",title:"Instances"},{path:"/admin/instances/:id",component:"AdminInstance",title:"Instance Detail"}];let u=null,p={};function $(t){t=t||"/",t!=="/"&&t.endsWith("/")&&(t=t.slice(0,-1));for(const e of N){const n={};let a=e.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");a=a.replace(/:[^/]+/g,"([^/]+)");const s=new RegExp(`^${a}$`),o=t.match(s);if(o)return(e.path.match(/:[^/]+/g)||[]).map(E=>E.slice(1)).forEach((E,F)=>{n[E]=decodeURIComponent(o[F+1])}),{route:e,params:n}}return null}function h(t,e={}){t.startsWith("/")||(t="/"+t);const n=$(t);if(!n){console.warn(`No route found for path: ${t}, falling back to list`),t="/blog-post";const a=$(t);a&&(u=a.route,p=a.params,window.history.pushState(e,"",t),C());return}if(n.route.roles&&n.route.roles.length>0){const a=U();if(!a||!q(a,n.route.roles)){console.warn("Access denied:",t),h("/blog-post");return}}u=n.route,p=n.params,window.history.pushState(e,"",t),C()}window.addEventListener("popstate",()=>{const t=window.location.pathname,e=$(t);e?(u=e.route,p=e.params,C()):h("/blog-post")});function U(){const t=localStorage.getItem("auth");if(t)try{return JSON.parse(t).user}catch{return null}return null}function q(t,e){return!t||!t.roles?!1:e.some(n=>t.roles.includes(n))}function C(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:u,params:p}}))}function z(){return p}function x(){return u}function V(){const t=window.location.pathname,e=$(t);e?(u=e.route,p=e.params):(u=N.find(n=>n.path==="/blog-post")||N[0],p={})}const S={brand:"blog-post",items:[{label:"blog-post",path:"/blog-post",icon:""},{label:"New",path:"/blog-post/new",icon:"+"},{label:"Admin",path:"/admin",icon:""}]};let i=null,L=!1;async function R(){if(!L){L=!0;try{const t={},e=j();e&&(t.Authorization=`Bearer ${e}`);const n=await fetch("/api/navigation",{headers:t});n.ok?i=await n.json():i=S}catch{i=S}finally{L=!1}}}async function M(){i||await R();const t=window.location.pathname,e=K(),n=(i==null?void 0:i.items)||S.items;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="/blog-post" onclick="handleNavClick(event, '/blog-post')">
          ${(i==null?void 0:i.brand)||S.brand}
        </a>
      </div>
      <ul class="nav-menu">
        ${n.map(o=>`
            <li class="${t===o.path||o.path!=="/"&&t.startsWith(o.path)?"active":""}">
              <a href="${o.path}" onclick="handleNavClick(event, '${o.path}')">
                ${o.icon?`<span class="icon">${o.icon}</span>`:""}
                ${o.label}
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
  `}window.handleNavClick=function(t,e){t.preventDefault(),h(e)};window.handleLogout=async function(){try{const t=j();t&&await fetch("/auth/logout",{method:"POST",headers:{Authorization:`Bearer ${t}`}})}catch(t){console.error("Logout error:",t)}localStorage.removeItem("auth"),i=null,window.dispatchEvent(new CustomEvent("auth-change")),await A(),h("/blog-post")};function K(){const t=localStorage.getItem("auth");if(t)try{return JSON.parse(t).user}catch{return null}return null}function j(){const t=localStorage.getItem("auth");if(t)try{return JSON.parse(t).token}catch{return null}return null}async function A(){i=null,await R();const t=document.getElementById("nav");t&&(t.innerHTML=await M())}window.addEventListener("auth-change",async()=>{await A()});window.addEventListener("route-change",()=>{const t=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(e=>{e.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(e=>{const n=e.getAttribute("href");(n===t||n!=="/"&&t.startsWith(n))&&e.parentElement.classList.add("active")})});let B=[];async function G(){try{const t=await fetch("/api/views");return t.ok?(B=await t.json(),B):(console.warn("Failed to load view definitions, using defaults"),[])}catch(t){return console.error("Error loading views:",t),[]}}const l="";let T=null,m=null,f=[],r=null;function Q(){const t=localStorage.getItem("auth");if(t)try{const e=JSON.parse(t);if(e.expires_at&&new Date(e.expires_at)>new Date)return m=e.token,T=e.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function H(t){localStorage.setItem("auth",JSON.stringify(t)),m=t.token,T=t.user,window.dispatchEvent(new CustomEvent("auth-change"))}function I(){localStorage.removeItem("auth"),m=null,T=null,window.dispatchEvent(new CustomEvent("auth-change"))}function d(){const t={"Content-Type":"application/json"};return m&&(t.Authorization=`Bearer ${m}`),t}async function b(t){if(t.status===401)throw I(),w("Session expired. Please log in again."),new Error("Unauthorized");if(!t.ok){const e=await t.json().catch(()=>({}));throw new Error(e.message||t.statusText)}return t.json()}const g={async getMe(){const t=await fetch(`${l}/auth/me`,{headers:d()});return b(t)},async logout(){await fetch(`${l}/auth/logout`,{method:"POST",headers:d()}),I()},async listInstances(){const t=await fetch(`${l}/admin/instances`,{headers:d()});return b(t)},async getInstance(t){const e=await fetch(`${l}/api/blogpost/${t}`,{headers:d()});return b(e)},async createInstance(t={}){const e=await fetch(`${l}/api/blogpost`,{method:"POST",headers:d(),body:JSON.stringify(t)});return b(e)},async executeTransition(t,e,n={}){const a=await fetch(`${l}/api/${t}`,{method:"POST",headers:d(),body:JSON.stringify({aggregate_id:e,data:n})});return b(a)}};window.api=g;window.setAuthToken=function(t){m=t};window.saveAuth=H;window.clearAuth=I;function w(t){const e=document.getElementById("app"),n=e.querySelector(".alert-error");n&&n.remove();const a=document.createElement("div");a.className="alert alert-error",a.textContent=t,e.insertBefore(a,e.firstChild),setTimeout(()=>a.remove(),5e3)}function _(t){const e=document.getElementById("app"),n=e.querySelector(".alert-success");n&&n.remove();const a=document.createElement("div");a.className="alert alert-success",a.textContent=t,e.insertBefore(a,e.firstChild),setTimeout(()=>a.remove(),3e3)}function O(t){if(!t)return"unknown";for(const[e,n]of Object.entries(t))if(n>0)return e;return"unknown"}function P(t){return`<span class="badge ${`badge-${t.toLowerCase().replace(/_/g,"-")}`}">${t.replace(/_/g," ")}</span>`}async function k(){const t=document.getElementById("app");t.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>blog-post</h1>
        <button class="btn btn-primary" onclick="handleCreateNew()">+ New</button>
      </div>
      <div id="instances-list" class="entity-list">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{f=(await g.listInstances()).instances||[],X()}catch{document.getElementById("instances-list").innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `}}function X(){const t=document.getElementById("instances-list");if(t){if(f.length===0){t.innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `;return}t.innerHTML=f.map(e=>{const n=O(e.state||e.places);return`
      <div class="entity-card" onclick="navigate('/blog-post/${e.id}')">
        <div class="entity-info">
          <h3>${e.id}</h3>
          <div class="entity-meta">
            ${P(n)} &middot; Version ${e.version||0}
          </div>
        </div>
        <div class="entity-actions">
          <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); navigate('/blog-post/${e.id}')">
            View
          </button>
        </div>
      </div>
    `}).join("")}}async function Y(){const e=z().id,n=document.getElementById("app");n.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/blog-post')" style="margin-left: -0.5rem">
            &larr; Back to List
          </button>
          <h1 style="margin-top: 0.5rem">Instance: ${e}</h1>
        </div>
      </div>
      <div id="instance-detail">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{const a=await g.getInstance(e);r={id:a.aggregate_id||e,version:a.version,state:a.state,places:a.places,enabled:a.enabled||a.enabled_transitions||[]},J()}catch(a){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${a.message}</div>
    `}}function J(){const t=document.getElementById("instance-detail");if(!t||!r)return;const e=O(r.places),n=r.enabled||[],a=[{id:"submit",name:"Submit",description:"Submit draft for review"},{id:"approve",name:"Approve",description:"Approve and publish the post"},{id:"reject",name:"Reject",description:"Reject and return to draft"},{id:"unpublish",name:"Unpublish",description:"Take down a published post"},{id:"restore",name:"Restore",description:"Restore archived post to draft"}];t.innerHTML=`
    <div class="card">
      <div class="card-header">Status</div>
      <div class="detail-list">
        <div class="detail-field">
          <dt>ID</dt>
          <dd><code>${r.id}</code></dd>
        </div>
        <div class="detail-field">
          <dt>Status</dt>
          <dd>${P(e)}</dd>
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
        ${a.map(s=>{const o=n.includes(s.id);return`
            <button
              class="btn ${o?"btn-primary":"btn-secondary"}"
              onclick="handleTransition('${s.id}')"
              ${o?"":"disabled"}
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
        ${Object.entries(r.places||{}).map(([s,o])=>`
          <div class="detail-field">
            <dt>${s}</dt>
            <dd>${o>0?`<span class="badge badge-${s}">${o} token${o>1?"s":""}</span>`:'<span style="color: #999;">0</span>'}</dd>
          </div>
        `).join("")}
      </div>
    </div>
  `}async function Z(){const t=document.getElementById("app");t.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/blog-post')" style="margin-left: -0.5rem">
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
            <button type="button" class="btn btn-secondary" onclick="navigate('/blog-post')">Cancel</button>
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
  `;try{const[e,n]=await Promise.all([fetch(`${l}/admin/stats`,{headers:d()}).then(s=>s.json()).catch(()=>null),g.listInstances()]);e?document.getElementById("admin-stats").innerHTML=`
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
              ${f.slice(0,20).map(s=>{const o=O(s.state||s.places);return`
                  <tr>
                    <td><code>${s.id}</code></td>
                    <td>${P(o)}</td>
                    <td>${s.version||0}</td>
                    <td><button class="btn btn-sm btn-link" onclick="navigate('/blog-post/${s.id}')">View</button></td>
                  </tr>
                `}).join("")}
            </tbody>
          </table>`:'<p style="color: #666; padding: 1rem;">No instances yet.</p>')}catch(e){w("Failed to load admin data: "+e.message)}}window.navigate=h;window.handleCreateNew=async function(){h("/blog-post/new")};window.handleSubmitCreate=async function(t){t.preventDefault();try{const e=await g.createInstance({});_("Instance created successfully!"),h(`/blog-post/${e.aggregate_id||e.id}`)}catch(e){w("Failed to create: "+e.message)}};window.handleTransition=async function(t){if(r)try{const e=await g.executeTransition(t,r.id);r={...r,version:e.version,state:e.state,places:e.state,enabled:e.enabled||[]},J(),_(`Action "${t}" completed!`)}catch(e){w(`Failed to execute ${t}: ${e.message}`)}};function D(t){var a;const e=((a=t.detail)==null?void 0:a.route)||x();if(!e){k();return}const n=e.path;n==="/blog-post"||n==="/"?k():n==="/blog-post/new"?Z():n==="/blog-post/:id"?Y():n==="/admin"||n.startsWith("/admin")?tt():k()}async function et(){const t=new URLSearchParams(window.location.search),e=t.get("token"),n=t.get("expires_at");if(e){m=e;try{const a=await g.getMe();H({token:e,expires_at:n,user:a}),window.history.replaceState({},"",window.location.pathname),await A()}catch{I(),w("Failed to complete login")}}}async function nt(){Q(),await et(),await G();const t=document.getElementById("nav");t.innerHTML=await M(),window.addEventListener("route-change",D),V(),D({detail:{route:x()}})}let c=null,y=null;function W(){const e=`${window.location.protocol==="https:"?"wss:":"ws:"}//${window.location.host}/ws`;c=new WebSocket(e),c.onopen=()=>{console.log("[Debug] WebSocket connected")},c.onmessage=n=>{try{const a=JSON.parse(n.data);a.id==="session"&&a.type==="session"?(y=(typeof a.data=="string"?JSON.parse(a.data):a.data).session_id,console.log("[Debug] Session ID:",y)):a.type==="eval"&&at(a)}catch(a){console.error("[Debug] Failed to parse message:",a)}},c.onclose=()=>{console.log("[Debug] WebSocket disconnected, reconnecting in 3s..."),y=null,setTimeout(W,3e3)},c.onerror=n=>{console.error("[Debug] WebSocket error:",n)}}async function at(t){try{const n=(typeof t.data=="string"?JSON.parse(t.data):t.data).code,s=await new Function("return (async () => { "+n+" })()")(),o={type:"response",id:t.id,data:{result:s,type:typeof s}};c.send(JSON.stringify(o))}catch(e){const n={type:"response",id:t.id,data:{error:e.message}};c.send(JSON.stringify(n))}}window.debugSessionId=()=>y;window.debugWs=()=>c;nt();W();
