(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const o of document.querySelectorAll('link[rel="modulepreload"]'))a(o);new MutationObserver(o=>{for(const r of o)if(r.type==="childList")for(const d of r.addedNodes)d.tagName==="LINK"&&d.rel==="modulepreload"&&a(d)}).observe(document,{childList:!0,subtree:!0});function n(o){const r={};return o.integrity&&(r.integrity=o.integrity),o.referrerPolicy&&(r.referrerPolicy=o.referrerPolicy),o.crossOrigin==="use-credentials"?r.credentials="include":o.crossOrigin==="anonymous"?r.credentials="omit":r.credentials="same-origin",r}function a(o){if(o.ep)return;o.ep=!0;const r=n(o);fetch(o.href,r)}})();const _=[{path:"/",component:"List",title:"loan-application"},{path:"/loan-application",component:"List",title:"loan-application"},{path:"/loan-application/new",component:"Form",title:"New loan-application"},{path:"/loan-application/:id",component:"Detail",title:"loan-application Detail"},{path:"/admin",component:"AdminDashboard",title:"Admin Dashboard"},{path:"/admin/instances",component:"AdminInstances",title:"Instances"},{path:"/admin/instances/:id",component:"AdminInstance",title:"Instance Detail"}];let v=null,y={};function A(e){e=e||"/",e!=="/"&&e.endsWith("/")&&(e=e.slice(0,-1));for(const t of _){const n={};let a=t.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");a=a.replace(/:[^/]+/g,"([^/]+)");const o=new RegExp(`^${a}$`),r=e.match(o);if(r)return(t.path.match(/:[^/]+/g)||[]).map(m=>m.slice(1)).forEach((m,H)=>{n[m]=decodeURIComponent(r[H+1])}),{route:t,params:n}}return null}function c(e,t={}){e.startsWith("/")||(e="/"+e);const n=A(e);if(!n){console.warn(`No route found for path: ${e}, falling back to list`),e="/loan-application";const a=A(e);a&&(v=a.route,y=a.params,window.history.pushState(t,"",e),L());return}if(n.route.roles&&n.route.roles.length>0){const a=J();if(!a||!W(a,n.route.roles)){console.warn("Access denied:",e),c("/loan-application");return}}v=n.route,y=n.params,window.history.pushState(t,"",e),L()}window.addEventListener("popstate",()=>{const e=window.location.pathname,t=A(e);t?(v=t.route,y=t.params,L()):c("/loan-application")});function J(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function W(e,t){return!e||!e.roles?!1:t.some(n=>e.roles.includes(n))}function L(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:v,params:y}}))}function z(){return y}function $(){return v}function V(){const e=window.location.pathname,t=A(e);t?(v=t.route,y=t.params):(v=_.find(n=>n.path==="/loan-application")||_[0],y={})}const C={brand:"loan-application",items:[{label:"loan-application",path:"/loan-application",icon:""},{label:"New",path:"/loan-application/new",icon:"+"},{label:"Admin",path:"/admin",icon:""}]};let s=null,T=!1;async function B(){if(!T){T=!0;try{const e={},t=j();t&&(e.Authorization=`Bearer ${t}`);const n=await fetch("/api/navigation",{headers:e});n.ok?s=await n.json():s=C}catch{s=C}finally{T=!1}}}async function M(){s||await B();const e=window.location.pathname,t=K(),n=(s==null?void 0:s.items)||C.items;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="/loan-application" onclick="handleNavClick(event, '/loan-application')">
          ${(s==null?void 0:s.brand)||C.brand}
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
  `}window.handleNavClick=function(e,t){e.preventDefault(),c(t)};window.handleLogout=async function(){try{const e=j();e&&await fetch("/auth/logout",{method:"POST",headers:{Authorization:`Bearer ${e}`}})}catch(e){console.error("Logout error:",e)}localStorage.removeItem("auth"),s=null,window.dispatchEvent(new CustomEvent("auth-change")),await R(),c("/loan-application")};function K(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function j(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).token}catch{return null}return null}async function R(){s=null,await B();const e=document.getElementById("nav");e&&(e.innerHTML=await M())}window.addEventListener("auth-change",async()=>{await R()});window.addEventListener("route-change",()=>{const e=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(t=>{t.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(t=>{const n=t.getAttribute("href");(n===e||n!=="/"&&e.startsWith(n))&&t.parentElement.classList.add("active")})});let P=[];async function G(){try{const e=await fetch("/api/views");return e.ok?(P=await e.json(),P):(console.warn("Failed to load view definitions, using defaults"),[])}catch(e){return console.error("Error loading views:",e),[]}}const u="";let p=null,g=null,w=[],i=null;function Q(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);if(t.expires_at&&new Date(t.expires_at)>new Date)return g=t.token,p=t.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function D(e){localStorage.setItem("auth",JSON.stringify(e)),g=e.token,p=e.user,window.dispatchEvent(new CustomEvent("auth-change"))}function E(){localStorage.removeItem("auth"),g=null,p=null,window.dispatchEvent(new CustomEvent("auth-change"))}function h(){const e={"Content-Type":"application/json"};return g&&(e.Authorization=`Bearer ${g}`),e}async function b(e){if(e.status===401)throw E(),S("Session expired. Please log in again."),new Error("Unauthorized");if(!e.ok){const t=await e.json().catch(()=>({}));throw new Error(t.message||e.statusText)}return e.json()}const l={async getMe(){const e=await fetch(`${u}/auth/me`,{headers:h()});return b(e)},async logout(){await fetch(`${u}/auth/logout`,{method:"POST",headers:h()}),E()},async listInstances(){const e=await fetch(`${u}/admin/instances`,{headers:h()});return b(e)},async getInstance(e){const t=await fetch(`${u}/api/loanapplication/${e}`,{headers:h()});return b(t)},async createInstance(e={}){const t=await fetch(`${u}/api/loanapplication`,{method:"POST",headers:h(),body:JSON.stringify(e)});return b(t)},async executeTransition(e,t,n={}){const a=await fetch(`${u}/api/${e}`,{method:"POST",headers:h(),body:JSON.stringify({aggregate_id:t,data:n})});return b(a)}};window.api=l;window.setAuthToken=function(e){g=e};window.saveAuth=D;window.clearAuth=E;function S(e){const t=document.getElementById("app"),n=t.querySelector(".alert-error");n&&n.remove();const a=document.createElement("div");a.className="alert alert-error",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),5e3)}function q(e){const t=document.getElementById("app"),n=t.querySelector(".alert-success");n&&n.remove();const a=document.createElement("div");a.className="alert alert-success",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),3e3)}function I(e){if(!e)return"unknown";for(const[t,n]of Object.entries(e))if(n>0)return t;return"unknown"}function x(e){return`<span class="badge ${`badge-${e.toLowerCase().replace(/_/g,"-")}`}">${e.replace(/_/g," ")}</span>`}async function F(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>loan-application</h1>
        <button class="btn btn-primary" onclick="handleCreateNew()">+ New</button>
      </div>
      <div id="instances-list" class="entity-list">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{w=(await l.listInstances()).instances||[],X()}catch{document.getElementById("instances-list").innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `}}function X(){const e=document.getElementById("instances-list");if(e){if(w.length===0){e.innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `;return}e.innerHTML=w.map(t=>{const n=I(t.state||t.places);return`
      <div class="entity-card" onclick="navigate('/loan-application/${t.id}')">
        <div class="entity-info">
          <h3>${t.id}</h3>
          <div class="entity-meta">
            ${x(n)} &middot; Version ${t.version||0}
          </div>
        </div>
        <div class="entity-actions">
          <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); navigate('/loan-application/${t.id}')">
            View
          </button>
        </div>
      </div>
    `}).join("")}}async function Y(){const t=z().id,n=document.getElementById("app");n.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/loan-application')" style="margin-left: -0.5rem">
            &larr; Back to List
          </button>
          <h1 style="margin-top: 0.5rem">Instance: ${t}</h1>
        </div>
      </div>
      <div id="instance-detail">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{const a=await l.getInstance(t);i={id:a.aggregate_id||t,version:a.version,state:a.state,places:a.places,enabled:a.enabled||a.enabled_transitions||[]},N()}catch(a){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${a.message}</div>
    `}}function N(){const e=document.getElementById("instance-detail");if(!e||!i)return;const t=I(i.places),n=i.enabled||[],a=[{id:"run_credit_check",name:"Run Credit Check",description:"Initiate automated credit check"},{id:"auto_approve",name:"Auto Approve",description:"Automatic approval based on credit score"},{id:"flag_for_review",name:"Flag For Review",description:"Flag application for manual review"},{id:"underwriter_approve",name:"Underwriter Approve",description:"Underwriter approves the application"},{id:"underwriter_deny",name:"Underwriter Deny",description:"Underwriter denies the application"},{id:"auto_deny",name:"Auto Deny",description:"Automatic denial based on credit score"},{id:"finalize_approval",name:"Finalize Approval",description:"Finalize loan approval"},{id:"disburse",name:"Disburse",description:"Disburse loan funds to customer"},{id:"start_repayment",name:"Start Repayment",description:"Begin repayment period"},{id:"make_payment",name:"Make Payment",description:"Customer makes a payment"},{id:"complete",name:"Complete",description:"Final payment received, loan complete"},{id:"mark_default",name:"Mark Default",description:"Mark loan as defaulted"}];e.innerHTML=`
    <div class="card">
      <div class="card-header">Status</div>
      <div class="detail-list">
        <div class="detail-field">
          <dt>ID</dt>
          <dd><code>${i.id}</code></dd>
        </div>
        <div class="detail-field">
          <dt>Status</dt>
          <dd>${x(t)}</dd>
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
        ${a.map(o=>{const r=n.includes(o.id);return`
            <button
              class="btn ${r?"btn-primary":"btn-secondary"}"
              onclick="handleTransition('${o.id}')"
              ${r?"":"disabled"}
              title="${o.description||o.name}"
            >
              ${o.name}
            </button>
          `}).join("")}
      </div>
      ${n.length===0?'<p style="color: #666; margin-top: 1rem;">No actions available in current state.</p>':""}
    </div>

    <div class="card">
      <div class="card-header">Current State</div>
      <div class="detail-list">
        ${Object.entries(i.places||{}).map(([o,r])=>`
          <div class="detail-field">
            <dt>${o}</dt>
            <dd>${r>0?`<span class="badge badge-${o}">${r} token${r>1?"s":""}</span>`:'<span style="color: #999;">0</span>'}</dd>
          </div>
        `).join("")}
      </div>
    </div>
  `}async function Z(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/loan-application')" style="margin-left: -0.5rem">
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
            <button type="button" class="btn btn-secondary" onclick="navigate('/loan-application')">Cancel</button>
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
  `;try{const[t,n]=await Promise.all([fetch(`${u}/admin/stats`,{headers:h()}).then(o=>o.json()).catch(()=>null),l.listInstances()]);t?document.getElementById("admin-stats").innerHTML=`
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
      `:document.getElementById("admin-stats").innerHTML="",w=n.instances||[];const a=document.getElementById("admin-instances").querySelector(".loading");a&&(a.outerHTML=w.length>0?`<table class="table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Status</th>
                <th>Version</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              ${w.slice(0,20).map(o=>{const r=I(o.state||o.places);return`
                  <tr>
                    <td><code>${o.id}</code></td>
                    <td>${x(r)}</td>
                    <td>${o.version||0}</td>
                    <td><button class="btn btn-sm btn-link" onclick="navigate('/loan-application/${o.id}')">View</button></td>
                  </tr>
                `}).join("")}
            </tbody>
          </table>`:'<p style="color: #666; padding: 1rem;">No instances yet.</p>')}catch(t){S("Failed to load admin data: "+t.message)}}window.navigate=c;window.handleCreateNew=async function(){c("/loan-application/new")};window.handleSubmitCreate=async function(e){e.preventDefault();try{const t=await l.createInstance({});q("Instance created successfully!"),c(`/loan-application/${t.aggregate_id||t.id}`)}catch(t){S("Failed to create: "+t.message)}};window.handleTransition=async function(e){if(i)try{const t=await l.executeTransition(e,i.id);i={...i,version:t.version,state:t.state,places:t.state,enabled:t.enabled||[]},N(),q(`Action "${e}" completed!`)}catch(t){S(`Failed to execute ${e}: ${t.message}`)}};function O(e){var a;const t=((a=e.detail)==null?void 0:a.route)||$();if(!t){F();return}const n=t.path;n==="/loan-application"||n==="/"?F():n==="/loan-application/new"?Z():n==="/loan-application/:id"?Y():n==="/admin"||n.startsWith("/admin")?ee():F()}async function te(){const e=new URLSearchParams(window.location.search),t=e.get("token"),n=e.get("expires_at");if(t){g=t;try{const a=await l.getMe();D({token:t,expires_at:n,user:a}),window.history.replaceState({},"",window.location.pathname),await R()}catch{E(),S("Failed to complete login")}}}async function ne(){Q(),await te(),await G();const e=document.getElementById("nav");e.innerHTML=await M(),window.addEventListener("route-change",O),V(),O({detail:{route:$()}})}let f=null,k=null;function U(){const t=`${window.location.protocol==="https:"?"wss:":"ws:"}//${window.location.host}/ws`;f=new WebSocket(t),f.onopen=()=>{console.log("[Debug] WebSocket connected")},f.onmessage=n=>{try{const a=JSON.parse(n.data);a.id==="session"&&a.type==="session"?(k=(typeof a.data=="string"?JSON.parse(a.data):a.data).session_id,console.log("[Debug] Session ID:",k)):a.type==="eval"&&ae(a)}catch(a){console.error("[Debug] Failed to parse message:",a)}},f.onclose=()=>{console.log("[Debug] WebSocket disconnected, reconnecting in 3s..."),k=null,setTimeout(U,3e3)},f.onerror=n=>{console.error("[Debug] WebSocket error:",n)}}async function ae(e){try{const n=(typeof e.data=="string"?JSON.parse(e.data):e.data).code,o=await new Function("return (async () => { "+n+" })()")(),r={type:"response",id:e.id,data:{result:o,type:typeof o}};f.send(JSON.stringify(r))}catch(t){const n={type:"response",id:e.id,data:{error:t.message}};f.send(JSON.stringify(n))}}window.debugSessionId=()=>k;window.debugWs=()=>f;window.pilot={list(){return c("/loan-application"),this.waitForRender()},newForm(){return c("/loan-application/new"),this.waitForRender()},async view(e){return c(`/loan-application/${e}`),await this.waitForRender(),i},admin(){return c("/admin"),this.waitForRender()},async create(e={}){const t=await l.createInstance(e),n=t.aggregate_id||t.id;return c(`/loan-application/${n}`),await this.waitForRender(),{id:n,...t}},getCurrentInstance(){return i},getInstances(){return w},async refresh(){if(!i)throw new Error("No current instance");const e=await l.getInstance(i.id);return i={id:e.aggregate_id||i.id,version:e.version,state:e.state,places:e.places,enabled:e.enabled||e.enabled_transitions||[]},N(),i},async action(e,t={}){if(!i)throw new Error("No current instance - navigate to detail page first");const n=await l.executeTransition(e,i.id,t);return i={...i,version:n.version,state:n.state,places:n.state,enabled:n.enabled||[]},N(),{success:!0,state:i.places,enabled:i.enabled}},isEnabled(e){return i?(i.enabled||[]).includes(e):!1},getEnabled(){return(i==null?void 0:i.enabled)||[]},fill(e,t){const n=document.querySelector(`[name="${e}"]`);if(!n)throw new Error(`No input found with name: ${e}`);return n.value=t,n.dispatchEvent(new Event("input",{bubbles:!0})),this},async submit(){const e=document.querySelector("form");if(!e)throw new Error("No form found on page");const t=new Event("submit",{bubbles:!0,cancelable:!0});return e.dispatchEvent(t),await this.waitForRender(),i},getText(e){const t=document.querySelector(e);return t?t.textContent.trim():null},exists(e){return document.querySelector(e)!==null},getButtons(){return Array.from(document.querySelectorAll("button")).map(e=>({text:e.textContent.trim(),disabled:e.disabled,className:e.className}))},async clickButton(e){const t=document.querySelectorAll("button");for(const n of t)if(n.textContent.trim()===e&&!n.disabled)return n.click(),await this.waitForRender(),!0;throw new Error(`No enabled button found with text: ${e}`)},getState(){return(i==null?void 0:i.places)||null},getStatus(){if(!(i!=null&&i.places))return null;for(const[e,t]of Object.entries(i.places))if(t>0)return e;return null},getRoute(){return $()},getUser(){return p},isAuthenticated(){return g!==null},waitForRender(e=50){return new Promise(t=>setTimeout(t,e))},async waitFor(e,t=5e3){const n=Date.now();for(;Date.now()-n<t;){if(document.querySelector(e))return document.querySelector(e);await this.waitForRender(50)}throw new Error(`Timeout waiting for: ${e}`)},async waitForState(e,t=5e3){var a;const n=Date.now();for(;Date.now()-n<t;){if(((a=i==null?void 0:i.places)==null?void 0:a[e])>0)return i;await this.waitForRender(100)}throw new Error(`Timeout waiting for state: ${e}`)},debug(){return console.log("=== Pilot Debug ==="),console.log("Route:",$()),console.log("User:",p),console.log("Instance:",i),console.log("Enabled:",i==null?void 0:i.enabled),console.log("State:",i==null?void 0:i.places),{route:$(),user:p,instance:i}},async getEvents(){if(!i)throw new Error("No current instance");const e=await fetch(`${u}/api/loanapplication/${i.id}/events`,{headers:h()});return(await b(e)).events||[]},async getEventCount(){return(await this.getEvents()).length},async getLastEvent(){const e=await this.getEvents();return e.length>0?e[e.length-1]:null},async replayTo(e){if(!i)throw new Error("No current instance");const n=(await this.getEvents()).filter(o=>(o.version||o.sequence)<=e),a={};for(const o of n)o.state&&Object.assign(a,o.state);return{version:e,events:n,places:a}},async loginAs(e){const t=typeof e=="string"?[e]:e,a=await(await fetch(`${u}/api/debug/login`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({login:"pilot-user",roles:t})})).json();return D(a),await this.waitForRender(100),a},logout(){return E(),this.waitForRender()},getRoles(){return(p==null?void 0:p.roles)||[]},hasRole(e){return this.getRoles().includes(e)},assertState(e){const t=this.getStatus();if(t!==e)throw new Error(`Expected state '${e}', got '${t}'`);return this},assertEnabled(e){if(!this.isEnabled(e)){const t=this.getEnabled();throw new Error(`Expected '${e}' to be enabled. Enabled: [${t.join(", ")}]`)}return this},assertDisabled(e){if(this.isEnabled(e))throw new Error(`Expected '${e}' to be disabled, but it is enabled`);return this},assertExists(e){if(!this.exists(e))throw new Error(`Expected element '${e}' to exist`);return this},assertText(e,t){const n=this.getText(e);if(n!==t)throw new Error(`Expected '${e}' to contain '${t}', got '${n}'`);return this},assertAuthenticated(){if(!this.isAuthenticated())throw new Error("Expected user to be authenticated");return this},assertRole(e){if(!this.hasRole(e))throw new Error(`Expected user to have role '${e}'. Has: [${this.getRoles().join(", ")}]`);return this},getTransitions(){return[{id:"run_credit_check",name:"Run Credit Check",description:"Initiate automated credit check"},{id:"auto_approve",name:"Auto Approve",description:"Automatic approval based on credit score"},{id:"flag_for_review",name:"Flag For Review",description:"Flag application for manual review"},{id:"underwriter_approve",name:"Underwriter Approve",description:"Underwriter approves the application"},{id:"underwriter_deny",name:"Underwriter Deny",description:"Underwriter denies the application"},{id:"auto_deny",name:"Auto Deny",description:"Automatic denial based on credit score"},{id:"finalize_approval",name:"Finalize Approval",description:"Finalize loan approval"},{id:"disburse",name:"Disburse",description:"Disburse loan funds to customer"},{id:"start_repayment",name:"Start Repayment",description:"Begin repayment period"},{id:"make_payment",name:"Make Payment",description:"Customer makes a payment"},{id:"complete",name:"Complete",description:"Final payment received, loan complete"},{id:"mark_default",name:"Mark Default",description:"Mark loan as defaulted"}]},getPlaces(){return[{id:"submitted",name:"Submitted",initial:1},{id:"credit_check",name:"CreditCheck",initial:0},{id:"auto_approved",name:"AutoApproved",initial:0},{id:"manual_review",name:"ManualReview",initial:0},{id:"approved",name:"Approved",initial:0},{id:"denied",name:"Denied",initial:0},{id:"disbursed",name:"Disbursed",initial:0},{id:"repaying",name:"Repaying",initial:0},{id:"paid_off",name:"PaidOff",initial:0},{id:"defaulted",name:"Defaulted",initial:0}]},getTransition(e){return this.getTransitions().find(t=>t.id===e)||null},canFire(e){if(!this.getTransition(e))return{canFire:!1,reason:`Unknown transition: ${e}`};if(!i)return{canFire:!1,reason:"No current instance"};if(!this.isEnabled(e)){const a=this.getStatus();return{canFire:!1,reason:`Transition '${e}' not enabled in state '${a}'`,currentState:a,enabledTransitions:this.getEnabled()}}return{canFire:!0}},async sequence(e,t={}){const n=[],{stopOnError:a=!0,data:o={}}=t;for(const r of e){const d=this.canFire(r);if(!d.canFire){if(a)throw new Error(`Sequence failed at '${r}': ${d.reason}`);n.push({transition:r,success:!1,error:d.reason});continue}try{const m=await this.action(r,o[r]||{});n.push({transition:r,success:!0,state:m.state})}catch(m){if(a)throw m;n.push({transition:r,success:!1,error:m.message})}}return n},getWorkflowInfo(){var e;return{places:this.getPlaces(),transitions:this.getTransitions(),initialPlace:(e=this.getPlaces().find(t=>t.initial>0))==null?void 0:e.id}}};ne();U();
