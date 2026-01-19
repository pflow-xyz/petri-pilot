(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const r of document.querySelectorAll('link[rel="modulepreload"]'))s(r);new MutationObserver(r=>{for(const i of r)if(i.type==="childList")for(const l of i.addedNodes)l.tagName==="LINK"&&l.rel==="modulepreload"&&s(l)}).observe(document,{childList:!0,subtree:!0});function n(r){const i={};return r.integrity&&(i.integrity=r.integrity),r.referrerPolicy&&(i.referrerPolicy=r.referrerPolicy),r.crossOrigin==="use-credentials"?i.credentials="include":r.crossOrigin==="anonymous"?i.credentials="omit":i.credentials="same-origin",i}function s(r){if(r.ep)return;r.ep=!0;const i=n(r);fetch(r.href,i)}})();const R=[{path:"/",component:"List",title:"order-processing"},{path:"/order-processing",component:"List",title:"order-processing"},{path:"/order-processing/new",component:"Form",title:"New order-processing"},{path:"/order-processing/:id",component:"Detail",title:"order-processing Detail"},{path:"/admin",component:"AdminDashboard",title:"Admin Dashboard"},{path:"/admin/instances",component:"AdminInstances",title:"Instances"},{path:"/admin/instances/:id",component:"AdminInstance",title:"Instance Detail"}];let v=null,b={};function N(e){e=e||"/",e!=="/"&&e.endsWith("/")&&(e=e.slice(0,-1));for(const t of R){const n={};let s=t.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");s=s.replace(/:[^/]+/g,"([^/]+)");const r=new RegExp(`^${s}$`),i=e.match(r);if(i)return(t.path.match(/:[^/]+/g)||[]).map(p=>p.slice(1)).forEach((p,J)=>{n[p]=decodeURIComponent(i[J+1])}),{route:t,params:n}}return null}function c(e,t={}){e.startsWith("/")||(e="/"+e);const n=N(e);if(!n){console.warn(`No route found for path: ${e}, falling back to list`),e="/order-processing";const s=N(e);s&&(v=s.route,b=s.params,window.history.pushState(t,"",e),I());return}if(n.route.roles&&n.route.roles.length>0){const s=W();if(!s||!U(s,n.route.roles)){console.warn("Access denied:",e),c("/order-processing");return}}v=n.route,b=n.params,window.history.pushState(t,"",e),I()}window.addEventListener("popstate",()=>{const e=window.location.pathname,t=N(e);t?(v=t.route,b=t.params,I()):c("/order-processing")});function W(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function U(e,t){return!e||!e.roles?!1:t.some(n=>e.roles.includes(n))}function I(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:v,params:b}}))}function V(){return b}function $(){return v}function z(){const e=window.location.pathname,t=N(e);t?(v=t.route,b=t.params):(v=R.find(n=>n.path==="/order-processing")||R[0],b={})}const C={brand:"order-processing",items:[{label:"order-processing",path:"/order-processing",icon:""},{label:"New",path:"/order-processing/new",icon:"+"},{label:"Admin",path:"/admin",icon:""}]};let o=null,L=!1;async function B(){if(!L){L=!0;try{const e={},t=_();t&&(e.Authorization=`Bearer ${t}`);const n=await fetch("/api/navigation",{headers:e});n.ok?o=await n.json():o=C}catch{o=C}finally{L=!1}}}async function M(){o||await B();const e=window.location.pathname,t=K(),n=(o==null?void 0:o.items)||C.items;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="/order-processing" onclick="handleNavClick(event, '/order-processing')">
          ${(o==null?void 0:o.brand)||C.brand}
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
  `}window.handleNavClick=function(e,t){e.preventDefault(),c(t)};window.handleLogout=async function(){try{const e=_();e&&await fetch("/auth/logout",{method:"POST",headers:{Authorization:`Bearer ${e}`}})}catch(e){console.error("Logout error:",e)}localStorage.removeItem("auth"),o=null,window.dispatchEvent(new CustomEvent("auth-change")),await x(),c("/order-processing")};function K(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function _(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).token}catch{return null}return null}async function x(){o=null,await B();const e=document.getElementById("nav");e&&(e.innerHTML=await M())}window.addEventListener("auth-change",async()=>{await x()});window.addEventListener("route-change",()=>{const e=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(t=>{t.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(t=>{const n=t.getAttribute("href");(n===e||n!=="/"&&e.startsWith(n))&&t.parentElement.classList.add("active")})});let D=[];async function G(){try{const e=await fetch("/api/views");return e.ok?(D=await e.json(),D):(console.warn("Failed to load view definitions, using defaults"),[])}catch(e){return console.error("Error loading views:",e),[]}}const u="";let h=null,f=null,w=[],a=null;function Q(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);if(t.expires_at&&new Date(t.expires_at)>new Date)return f=t.token,h=t.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function P(e){localStorage.setItem("auth",JSON.stringify(e)),f=e.token,h=e.user,window.dispatchEvent(new CustomEvent("auth-change"))}function E(){localStorage.removeItem("auth"),f=null,h=null,window.dispatchEvent(new CustomEvent("auth-change"))}function g(){const e={"Content-Type":"application/json"};return f&&(e.Authorization=`Bearer ${f}`),e}async function y(e){if(e.status===401)throw E(),S("Session expired. Please log in again."),new Error("Unauthorized");if(!e.ok){const t=await e.json().catch(()=>({}));throw new Error(t.message||e.statusText)}return e.json()}const d={async getMe(){const e=await fetch(`${u}/auth/me`,{headers:g()});return y(e)},async logout(){await fetch(`${u}/auth/logout`,{method:"POST",headers:g()}),E()},async listInstances(){const e=await fetch(`${u}/admin/instances`,{headers:g()});return y(e)},async getInstance(e){const t=await fetch(`${u}/api/orderprocessing/${e}`,{headers:g()});return y(t)},async createInstance(e={}){const t=await fetch(`${u}/api/orderprocessing`,{method:"POST",headers:g(),body:JSON.stringify(e)});return y(t)},async executeTransition(e,t,n={}){const s=await fetch(`${u}/api/${e}`,{method:"POST",headers:g(),body:JSON.stringify({aggregate_id:t,data:n})});return y(s)}};window.api=d;window.setAuthToken=function(e){f=e};window.saveAuth=P;window.clearAuth=E;function S(e){const t=document.getElementById("app"),n=t.querySelector(".alert-error");n&&n.remove();const s=document.createElement("div");s.className="alert alert-error",s.textContent=e,t.insertBefore(s,t.firstChild),setTimeout(()=>s.remove(),5e3)}function q(e){const t=document.getElementById("app"),n=t.querySelector(".alert-success");n&&n.remove();const s=document.createElement("div");s.className="alert alert-success",s.textContent=e,t.insertBefore(s,t.firstChild),setTimeout(()=>s.remove(),3e3)}function F(e){if(!e)return"unknown";for(const[t,n]of Object.entries(e))if(n>0)return t;return"unknown"}function O(e){return`<span class="badge ${`badge-${e.toLowerCase().replace(/_/g,"-")}`}">${e.replace(/_/g," ")}</span>`}async function A(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>order-processing</h1>
        <button class="btn btn-primary" onclick="handleCreateNew()">+ New</button>
      </div>
      <div id="instances-list" class="entity-list">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{w=(await d.listInstances()).instances||[],X()}catch{document.getElementById("instances-list").innerHTML=`
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
    `;return}e.innerHTML=w.map(t=>{const n=F(t.state||t.places);return`
      <div class="entity-card" onclick="navigate('/order-processing/${t.id}')">
        <div class="entity-info">
          <h3>${t.id}</h3>
          <div class="entity-meta">
            ${O(n)} &middot; Version ${t.version||0}
          </div>
        </div>
        <div class="entity-actions">
          <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); navigate('/order-processing/${t.id}')">
            View
          </button>
        </div>
      </div>
    `}).join("")}}async function Y(){const t=V().id,n=document.getElementById("app");n.innerHTML=`
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
  `;try{const s=await d.getInstance(t);a={id:s.aggregate_id||t,version:s.version,state:s.state,places:s.places,enabled:s.enabled||s.enabled_transitions||[]},T()}catch(s){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${s.message}</div>
    `}}function T(){const e=document.getElementById("instance-detail");if(!e||!a)return;const t=F(a.places),n=a.enabled||[],s=[{id:"validate",name:"Validate",description:"Check order validity"},{id:"reject",name:"Reject",description:"Mark order as invalid"},{id:"process_payment",name:"Process Payment",description:"Charge customer payment"},{id:"ship",name:"Ship",description:"Send order to shipping"},{id:"confirm",name:"Confirm",description:"Mark order as complete"}];e.innerHTML=`
    <div class="card">
      <div class="card-header">Status</div>
      <div class="detail-list">
        <div class="detail-field">
          <dt>ID</dt>
          <dd><code>${a.id}</code></dd>
        </div>
        <div class="detail-field">
          <dt>Status</dt>
          <dd>${O(t)}</dd>
        </div>
        <div class="detail-field">
          <dt>Version</dt>
          <dd>${a.version||0}</dd>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="card-header">Actions</div>
      <div class="view-actions">
        ${s.map(r=>{const i=n.includes(r.id);return`
            <button
              class="btn ${i?"btn-primary":"btn-secondary"}"
              onclick="handleTransition('${r.id}')"
              ${i?"":"disabled"}
              title="${r.description||r.name}"
            >
              ${r.name}
            </button>
          `}).join("")}
      </div>
      ${n.length===0?'<p style="color: #666; margin-top: 1rem;">No actions available in current state.</p>':""}
    </div>

    <div class="card">
      <div class="card-header">Current State</div>
      <div class="detail-list">
        ${Object.entries(a.places||{}).map(([r,i])=>`
          <div class="detail-field">
            <dt>${r}</dt>
            <dd>${i>0?`<span class="badge badge-${r}">${i} token${i>1?"s":""}</span>`:'<span style="color: #999;">0</span>'}</dd>
          </div>
        `).join("")}
      </div>
    </div>
  `}async function Z(){const e=document.getElementById("app");e.innerHTML=`
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
  `;try{const[t,n]=await Promise.all([fetch(`${u}/admin/stats`,{headers:g()}).then(r=>r.json()).catch(()=>null),d.listInstances()]);t?document.getElementById("admin-stats").innerHTML=`
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
      `:document.getElementById("admin-stats").innerHTML="",w=n.instances||[];const s=document.getElementById("admin-instances").querySelector(".loading");s&&(s.outerHTML=w.length>0?`<table class="table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Status</th>
                <th>Version</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              ${w.slice(0,20).map(r=>{const i=F(r.state||r.places);return`
                  <tr>
                    <td><code>${r.id}</code></td>
                    <td>${O(i)}</td>
                    <td>${r.version||0}</td>
                    <td><button class="btn btn-sm btn-link" onclick="navigate('/order-processing/${r.id}')">View</button></td>
                  </tr>
                `}).join("")}
            </tbody>
          </table>`:'<p style="color: #666; padding: 1rem;">No instances yet.</p>')}catch(t){S("Failed to load admin data: "+t.message)}}window.navigate=c;window.handleCreateNew=async function(){c("/order-processing/new")};window.handleSubmitCreate=async function(e){e.preventDefault();try{const t=await d.createInstance({});q("Instance created successfully!"),c(`/order-processing/${t.aggregate_id||t.id}`)}catch(t){S("Failed to create: "+t.message)}};window.handleTransition=async function(e){if(a)try{const t=await d.executeTransition(e,a.id);a={...a,version:t.version,state:t.state,places:t.state,enabled:t.enabled||[]},T(),q(`Action "${e}" completed!`)}catch(t){S(`Failed to execute ${e}: ${t.message}`)}};function j(e){var s;const t=((s=e.detail)==null?void 0:s.route)||$();if(!t){A();return}const n=t.path;n==="/order-processing"||n==="/"?A():n==="/order-processing/new"?Z():n==="/order-processing/:id"?Y():n==="/admin"||n.startsWith("/admin")?ee():A()}async function te(){const e=new URLSearchParams(window.location.search),t=e.get("token"),n=e.get("expires_at");if(t){f=t;try{const s=await d.getMe();P({token:t,expires_at:n,user:s}),window.history.replaceState({},"",window.location.pathname),await x()}catch{E(),S("Failed to complete login")}}}async function ne(){Q(),await te(),await G();const e=document.getElementById("nav");e.innerHTML=await M(),window.addEventListener("route-change",j),z(),j({detail:{route:$()}})}let m=null,k=null;function H(){const t=`${window.location.protocol==="https:"?"wss:":"ws:"}//${window.location.host}/ws`;m=new WebSocket(t),m.onopen=()=>{console.log("[Debug] WebSocket connected")},m.onmessage=n=>{try{const s=JSON.parse(n.data);s.id==="session"&&s.type==="session"?(k=(typeof s.data=="string"?JSON.parse(s.data):s.data).session_id,console.log("[Debug] Session ID:",k)):s.type==="eval"&&se(s)}catch(s){console.error("[Debug] Failed to parse message:",s)}},m.onclose=()=>{console.log("[Debug] WebSocket disconnected, reconnecting in 3s..."),k=null,setTimeout(H,3e3)},m.onerror=n=>{console.error("[Debug] WebSocket error:",n)}}async function se(e){try{const n=(typeof e.data=="string"?JSON.parse(e.data):e.data).code,r=await new Function("return (async () => { "+n+" })()")(),i={type:"response",id:e.id,data:{result:r,type:typeof r}};m.send(JSON.stringify(i))}catch(t){const n={type:"response",id:e.id,data:{error:t.message}};m.send(JSON.stringify(n))}}window.debugSessionId=()=>k;window.debugWs=()=>m;window.pilot={list(){return c("/order-processing"),this.waitForRender()},newForm(){return c("/order-processing/new"),this.waitForRender()},async view(e){return c(`/order-processing/${e}`),await this.waitForRender(),a},admin(){return c("/admin"),this.waitForRender()},async create(e={}){const t=await d.createInstance(e),n=t.aggregate_id||t.id;return c(`/order-processing/${n}`),await this.waitForRender(),{id:n,...t}},getCurrentInstance(){return a},getInstances(){return w},async refresh(){if(!a)throw new Error("No current instance");const e=await d.getInstance(a.id);return a={id:e.aggregate_id||a.id,version:e.version,state:e.state,places:e.places,enabled:e.enabled||e.enabled_transitions||[]},T(),a},async action(e,t={}){if(!a)throw new Error("No current instance - navigate to detail page first");const n=await d.executeTransition(e,a.id,t);return a={...a,version:n.version,state:n.state,places:n.state,enabled:n.enabled||[]},T(),{success:!0,state:a.places,enabled:a.enabled}},isEnabled(e){return a?(a.enabled||[]).includes(e):!1},getEnabled(){return(a==null?void 0:a.enabled)||[]},fill(e,t){const n=document.querySelector(`[name="${e}"]`);if(!n)throw new Error(`No input found with name: ${e}`);return n.value=t,n.dispatchEvent(new Event("input",{bubbles:!0})),this},async submit(){const e=document.querySelector("form");if(!e)throw new Error("No form found on page");const t=new Event("submit",{bubbles:!0,cancelable:!0});return e.dispatchEvent(t),await this.waitForRender(),a},getText(e){const t=document.querySelector(e);return t?t.textContent.trim():null},exists(e){return document.querySelector(e)!==null},getButtons(){return Array.from(document.querySelectorAll("button")).map(e=>({text:e.textContent.trim(),disabled:e.disabled,className:e.className}))},async clickButton(e){const t=document.querySelectorAll("button");for(const n of t)if(n.textContent.trim()===e&&!n.disabled)return n.click(),await this.waitForRender(),!0;throw new Error(`No enabled button found with text: ${e}`)},getState(){return(a==null?void 0:a.places)||null},getStatus(){if(!(a!=null&&a.places))return null;for(const[e,t]of Object.entries(a.places))if(t>0)return e;return null},getRoute(){return $()},getUser(){return h},isAuthenticated(){return f!==null},waitForRender(e=50){return new Promise(t=>setTimeout(t,e))},async waitFor(e,t=5e3){const n=Date.now();for(;Date.now()-n<t;){if(document.querySelector(e))return document.querySelector(e);await this.waitForRender(50)}throw new Error(`Timeout waiting for: ${e}`)},async waitForState(e,t=5e3){var s;const n=Date.now();for(;Date.now()-n<t;){if(((s=a==null?void 0:a.places)==null?void 0:s[e])>0)return a;await this.waitForRender(100)}throw new Error(`Timeout waiting for state: ${e}`)},debug(){return console.log("=== Pilot Debug ==="),console.log("Route:",$()),console.log("User:",h),console.log("Instance:",a),console.log("Enabled:",a==null?void 0:a.enabled),console.log("State:",a==null?void 0:a.places),{route:$(),user:h,instance:a}},async getEvents(){if(!a)throw new Error("No current instance");const e=await fetch(`${u}/api/orderprocessing/${a.id}/events`,{headers:g()});return(await y(e)).events||[]},async getEventCount(){return(await this.getEvents()).length},async getLastEvent(){const e=await this.getEvents();return e.length>0?e[e.length-1]:null},async replayTo(e){if(!a)throw new Error("No current instance");const n=(await this.getEvents()).filter(r=>(r.version||r.sequence)<=e),s={};for(const r of n)r.state&&Object.assign(s,r.state);return{version:e,events:n,places:s}},async loginAs(e){const t=typeof e=="string"?[e]:e,s=await(await fetch(`${u}/api/debug/login`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({login:"pilot-user",roles:t})})).json();return P(s),await this.waitForRender(100),s},logout(){return E(),this.waitForRender()},getRoles(){return(h==null?void 0:h.roles)||[]},hasRole(e){return this.getRoles().includes(e)},assertState(e){const t=this.getStatus();if(t!==e)throw new Error(`Expected state '${e}', got '${t}'`);return this},assertEnabled(e){if(!this.isEnabled(e)){const t=this.getEnabled();throw new Error(`Expected '${e}' to be enabled. Enabled: [${t.join(", ")}]`)}return this},assertDisabled(e){if(this.isEnabled(e))throw new Error(`Expected '${e}' to be disabled, but it is enabled`);return this},assertExists(e){if(!this.exists(e))throw new Error(`Expected element '${e}' to exist`);return this},assertText(e,t){const n=this.getText(e);if(n!==t)throw new Error(`Expected '${e}' to contain '${t}', got '${n}'`);return this},assertAuthenticated(){if(!this.isAuthenticated())throw new Error("Expected user to be authenticated");return this},assertRole(e){if(!this.hasRole(e))throw new Error(`Expected user to have role '${e}'. Has: [${this.getRoles().join(", ")}]`);return this},getTransitions(){return[{id:"validate",name:"Validate",description:"Check order validity"},{id:"reject",name:"Reject",description:"Mark order as invalid"},{id:"process_payment",name:"Process Payment",description:"Charge customer payment"},{id:"ship",name:"Ship",description:"Send order to shipping"},{id:"confirm",name:"Confirm",description:"Mark order as complete"}]},getPlaces(){return[{id:"received",name:"Received",initial:1},{id:"validated",name:"Validated",initial:0},{id:"rejected",name:"Rejected",initial:0},{id:"paid",name:"Paid",initial:0},{id:"shipped",name:"Shipped",initial:0},{id:"completed",name:"Completed",initial:0}]},getTransition(e){return this.getTransitions().find(t=>t.id===e)||null},canFire(e){if(!this.getTransition(e))return{canFire:!1,reason:`Unknown transition: ${e}`};if(!a)return{canFire:!1,reason:"No current instance"};if(!this.isEnabled(e)){const s=this.getStatus();return{canFire:!1,reason:`Transition '${e}' not enabled in state '${s}'`,currentState:s,enabledTransitions:this.getEnabled()}}return{canFire:!0}},async sequence(e,t={}){const n=[],{stopOnError:s=!0,data:r={}}=t;for(const i of e){const l=this.canFire(i);if(!l.canFire){if(s)throw new Error(`Sequence failed at '${i}': ${l.reason}`);n.push({transition:i,success:!1,error:l.reason});continue}try{const p=await this.action(i,r[i]||{});n.push({transition:i,success:!0,state:p.state})}catch(p){if(s)throw p;n.push({transition:i,success:!1,error:p.message})}}return n},getWorkflowInfo(){var e;return{places:this.getPlaces(),transitions:this.getTransitions(),initialPlace:(e=this.getPlaces().find(t=>t.initial>0))==null?void 0:e.id}}};ne();H();
