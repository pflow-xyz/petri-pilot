(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const s of document.querySelectorAll('link[rel="modulepreload"]'))a(s);new MutationObserver(s=>{for(const o of s)if(o.type==="childList")for(const g of o.addedNodes)g.tagName==="LINK"&&g.rel==="modulepreload"&&a(g)}).observe(document,{childList:!0,subtree:!0});function n(s){const o={};return s.integrity&&(o.integrity=s.integrity),s.referrerPolicy&&(o.referrerPolicy=s.referrerPolicy),s.crossOrigin==="use-credentials"?o.credentials="include":s.crossOrigin==="anonymous"?o.credentials="omit":o.credentials="same-origin",o}function a(s){if(s.ep)return;s.ep=!0;const o=n(s);fetch(s.href,o)}})();const C=[{path:"/",component:"List",title:"ecommerce-checkout"},{path:"/ecommerce-checkout",component:"List",title:"ecommerce-checkout"},{path:"/ecommerce-checkout/new",component:"Form",title:"New ecommerce-checkout"},{path:"/ecommerce-checkout/:id",component:"Detail",title:"ecommerce-checkout Detail"},{path:"/admin",component:"AdminDashboard",title:"Admin Dashboard"},{path:"/admin/instances",component:"AdminInstances",title:"Instances"},{path:"/admin/instances/:id",component:"AdminInstance",title:"Instance Detail"}];let u=null,m={};function k(e){e=e||"/",e!=="/"&&e.endsWith("/")&&(e=e.slice(0,-1));for(const t of C){const n={};let a=t.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");a=a.replace(/:[^/]+/g,"([^/]+)");const s=new RegExp(`^${a}$`),o=e.match(s);if(o)return(t.path.match(/:[^/]+/g)||[]).map(I=>I.slice(1)).forEach((I,W)=>{n[I]=decodeURIComponent(o[W+1])}),{route:t,params:n}}return null}function p(e,t={}){e.startsWith("/")||(e="/"+e);const n=k(e);if(!n){console.warn(`No route found for path: ${e}, falling back to list`),e="/ecommerce-checkout";const a=k(e);a&&(u=a.route,m=a.params,window.history.pushState(t,"",e),N());return}if(n.route.roles&&n.route.roles.length>0){const a=U();if(!a||!q(a,n.route.roles)){console.warn("Access denied:",e),p("/ecommerce-checkout");return}}u=n.route,m=n.params,window.history.pushState(t,"",e),N()}window.addEventListener("popstate",()=>{const e=window.location.pathname,t=k(e);t?(u=t.route,m=t.params,N()):p("/ecommerce-checkout")});function U(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function q(e,t){return!e||!e.roles?!1:t.some(n=>e.roles.includes(n))}function N(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:u,params:m}}))}function z(){return m}function D(){return u}function V(){const e=window.location.pathname,t=k(e);t?(u=t.route,m=t.params):(u=C.find(n=>n.path==="/ecommerce-checkout")||C[0],m={})}const $={brand:"ecommerce-checkout",items:[{label:"ecommerce-checkout",path:"/ecommerce-checkout",icon:""},{label:"New",path:"/ecommerce-checkout/new",icon:"+"},{label:"Admin",path:"/admin",icon:""}]};let c=null,E=!1;async function x(){if(!E){E=!0;try{const e={},t=F();t&&(e.Authorization=`Bearer ${t}`);const n=await fetch("/api/navigation",{headers:e});n.ok?c=await n.json():c=$}catch{c=$}finally{E=!1}}}async function R(){c||await x();const e=window.location.pathname,t=K(),n=(c==null?void 0:c.items)||$.items;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="/ecommerce-checkout" onclick="handleNavClick(event, '/ecommerce-checkout')">
          ${(c==null?void 0:c.brand)||$.brand}
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
  `}window.handleNavClick=function(e,t){e.preventDefault(),p(t)};window.handleLogout=async function(){try{const e=F();e&&await fetch("/auth/logout",{method:"POST",headers:{Authorization:`Bearer ${e}`}})}catch(e){console.error("Logout error:",e)}localStorage.removeItem("auth"),c=null,window.dispatchEvent(new CustomEvent("auth-change")),await P(),p("/ecommerce-checkout")};function K(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function F(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).token}catch{return null}return null}async function P(){c=null,await x();const e=document.getElementById("nav");e&&(e.innerHTML=await R())}window.addEventListener("auth-change",async()=>{await P()});window.addEventListener("route-change",()=>{const e=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(t=>{t.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(t=>{const n=t.getAttribute("href");(n===e||n!=="/"&&e.startsWith(n))&&t.parentElement.classList.add("active")})});let O=[];async function G(){try{const e=await fetch("/api/views");return e.ok?(O=await e.json(),O):(console.warn("Failed to load view definitions, using defaults"),[])}catch(e){return console.error("Error loading views:",e),[]}}const d="";let T=null,h=null,v=[],i=null;function Q(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);if(t.expires_at&&new Date(t.expires_at)>new Date)return h=t.token,T=t.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function M(e){localStorage.setItem("auth",JSON.stringify(e)),h=e.token,T=e.user,window.dispatchEvent(new CustomEvent("auth-change"))}function S(){localStorage.removeItem("auth"),h=null,T=null,window.dispatchEvent(new CustomEvent("auth-change"))}function l(){const e={"Content-Type":"application/json"};return h&&(e.Authorization=`Bearer ${h}`),e}async function y(e){if(e.status===401)throw S(),w("Session expired. Please log in again."),new Error("Unauthorized");if(!e.ok){const t=await e.json().catch(()=>({}));throw new Error(t.message||e.statusText)}return e.json()}const f={async getMe(){const e=await fetch(`${d}/auth/me`,{headers:l()});return y(e)},async logout(){await fetch(`${d}/auth/logout`,{method:"POST",headers:l()}),S()},async listInstances(){const e=await fetch(`${d}/admin/instances`,{headers:l()});return y(e)},async getInstance(e){const t=await fetch(`${d}/api/ecommercecheckout/${e}`,{headers:l()});return y(t)},async createInstance(e={}){const t=await fetch(`${d}/api/ecommercecheckout`,{method:"POST",headers:l(),body:JSON.stringify(e)});return y(t)},async executeTransition(e,t,n={}){const a=await fetch(`${d}/api/${e}`,{method:"POST",headers:l(),body:JSON.stringify({aggregate_id:t,data:n})});return y(a)}};window.api=f;window.setAuthToken=function(e){h=e};window.saveAuth=M;window.clearAuth=S;function w(e){const t=document.getElementById("app"),n=t.querySelector(".alert-error");n&&n.remove();const a=document.createElement("div");a.className="alert alert-error",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),5e3)}function H(e){const t=document.getElementById("app"),n=t.querySelector(".alert-success");n&&n.remove();const a=document.createElement("div");a.className="alert alert-success",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),3e3)}function A(e){if(!e)return"unknown";for(const[t,n]of Object.entries(e))if(n>0)return t;return"unknown"}function _(e){return`<span class="badge ${`badge-${e.toLowerCase().replace(/_/g,"-")}`}">${e.replace(/_/g," ")}</span>`}async function L(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>ecommerce-checkout</h1>
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
    `;return}e.innerHTML=v.map(t=>{const n=A(t.state||t.places);return`
      <div class="entity-card" onclick="navigate('/ecommerce-checkout/${t.id}')">
        <div class="entity-info">
          <h3>${t.id}</h3>
          <div class="entity-meta">
            ${_(n)} &middot; Version ${t.version||0}
          </div>
        </div>
        <div class="entity-actions">
          <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); navigate('/ecommerce-checkout/${t.id}')">
            View
          </button>
        </div>
      </div>
    `}).join("")}}async function Y(){const t=z().id,n=document.getElementById("app");n.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/ecommerce-checkout')" style="margin-left: -0.5rem">
            &larr; Back to List
          </button>
          <h1 style="margin-top: 0.5rem">Instance: ${t}</h1>
        </div>
      </div>
      <div id="instance-detail">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{const a=await f.getInstance(t);i={id:a.aggregate_id||t,version:a.version,state:a.state,places:a.places,enabled:a.enabled||a.enabled_transitions||[]},j()}catch(a){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${a.message}</div>
    `}}function j(){const e=document.getElementById("instance-detail");if(!e||!i)return;const t=A(i.places),n=i.enabled||[],a=[{id:"start_checkout",name:"Start Checkout",description:"Begin checkout process"},{id:"enter_payment",name:"Enter Payment",description:"Enter payment details"},{id:"process_payment",name:"Process Payment",description:"Process the payment"},{id:"payment_success",name:"Payment Success",description:"Payment processed successfully"},{id:"payment_fail_1",name:"Payment Fail 1",description:"First payment attempt failed"},{id:"retry_payment_1",name:"Retry Payment 1",description:"Retry payment (attempt 2)"},{id:"payment_fail_2",name:"Payment Fail 2",description:"Second payment attempt failed"},{id:"retry_payment_2",name:"Retry Payment 2",description:"Retry payment (attempt 3)"},{id:"payment_fail_3",name:"Payment Fail 3",description:"Third payment attempt failed"},{id:"cancel_order",name:"Cancel Order",description:"Cancel order after max retries"},{id:"fulfill",name:"Fulfill",description:"Fulfill the order"}];e.innerHTML=`
    <div class="card">
      <div class="card-header">Status</div>
      <div class="detail-list">
        <div class="detail-field">
          <dt>ID</dt>
          <dd><code>${i.id}</code></dd>
        </div>
        <div class="detail-field">
          <dt>Status</dt>
          <dd>${_(t)}</dd>
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
        ${Object.entries(i.places||{}).map(([s,o])=>`
          <div class="detail-field">
            <dt>${s}</dt>
            <dd>${o>0?`<span class="badge badge-${s}">${o} token${o>1?"s":""}</span>`:'<span style="color: #999;">0</span>'}</dd>
          </div>
        `).join("")}
      </div>
    </div>
  `}async function Z(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/ecommerce-checkout')" style="margin-left: -0.5rem">
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
            <button type="button" class="btn btn-secondary" onclick="navigate('/ecommerce-checkout')">Cancel</button>
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
  `;try{const[t,n]=await Promise.all([fetch(`${d}/admin/stats`,{headers:l()}).then(s=>s.json()).catch(()=>null),f.listInstances()]);t?document.getElementById("admin-stats").innerHTML=`
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
              ${v.slice(0,20).map(s=>{const o=A(s.state||s.places);return`
                  <tr>
                    <td><code>${s.id}</code></td>
                    <td>${_(o)}</td>
                    <td>${s.version||0}</td>
                    <td><button class="btn btn-sm btn-link" onclick="navigate('/ecommerce-checkout/${s.id}')">View</button></td>
                  </tr>
                `}).join("")}
            </tbody>
          </table>`:'<p style="color: #666; padding: 1rem;">No instances yet.</p>')}catch(t){w("Failed to load admin data: "+t.message)}}window.navigate=p;window.handleCreateNew=async function(){p("/ecommerce-checkout/new")};window.handleSubmitCreate=async function(e){e.preventDefault();try{const t=await f.createInstance({});H("Instance created successfully!"),p(`/ecommerce-checkout/${t.aggregate_id||t.id}`)}catch(t){w("Failed to create: "+t.message)}};window.handleTransition=async function(e){if(i)try{const t=await f.executeTransition(e,i.id);i={...i,version:t.version,state:t.state,places:t.state,enabled:t.enabled||[]},j(),H(`Action "${e}" completed!`)}catch(t){w(`Failed to execute ${e}: ${t.message}`)}};function B(e){var a;const t=((a=e.detail)==null?void 0:a.route)||D();if(!t){L();return}const n=t.path;n==="/ecommerce-checkout"||n==="/"?L():n==="/ecommerce-checkout/new"?Z():n==="/ecommerce-checkout/:id"?Y():n==="/admin"||n.startsWith("/admin")?ee():L()}async function te(){const e=new URLSearchParams(window.location.search),t=e.get("token"),n=e.get("expires_at");if(t){h=t;try{const a=await f.getMe();M({token:t,expires_at:n,user:a}),window.history.replaceState({},"",window.location.pathname),await P()}catch{S(),w("Failed to complete login")}}}async function ne(){Q(),await te(),await G();const e=document.getElementById("nav");e.innerHTML=await R(),window.addEventListener("route-change",B),V(),B({detail:{route:D()}})}let r=null,b=null;function J(){const t=`${window.location.protocol==="https:"?"wss:":"ws:"}//${window.location.host}/ws`;r=new WebSocket(t),r.onopen=()=>{console.log("[Debug] WebSocket connected")},r.onmessage=n=>{try{const a=JSON.parse(n.data);a.id==="session"&&a.type==="session"?(b=(typeof a.data=="string"?JSON.parse(a.data):a.data).session_id,console.log("[Debug] Session ID:",b)):a.type==="eval"&&ae(a)}catch(a){console.error("[Debug] Failed to parse message:",a)}},r.onclose=()=>{console.log("[Debug] WebSocket disconnected, reconnecting in 3s..."),b=null,setTimeout(J,3e3)},r.onerror=n=>{console.error("[Debug] WebSocket error:",n)}}async function ae(e){try{const n=(typeof e.data=="string"?JSON.parse(e.data):e.data).code,s=await new Function("return (async () => { "+n+" })()")(),o={type:"response",id:e.id,data:{result:s,type:typeof s}};r.send(JSON.stringify(o))}catch(t){const n={type:"response",id:e.id,data:{error:t.message}};r.send(JSON.stringify(n))}}window.debugSessionId=()=>b;window.debugWs=()=>r;ne();J();
