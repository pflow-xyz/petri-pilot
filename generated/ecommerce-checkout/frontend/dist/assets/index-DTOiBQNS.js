(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const i of document.querySelectorAll('link[rel="modulepreload"]'))a(i);new MutationObserver(i=>{for(const r of i)if(r.type==="childList")for(const d of r.addedNodes)d.tagName==="LINK"&&d.rel==="modulepreload"&&a(d)}).observe(document,{childList:!0,subtree:!0});function n(i){const r={};return i.integrity&&(r.integrity=i.integrity),i.referrerPolicy&&(r.referrerPolicy=i.referrerPolicy),i.crossOrigin==="use-credentials"?r.credentials="include":i.crossOrigin==="anonymous"?r.credentials="omit":r.credentials="same-origin",r}function a(i){if(i.ep)return;i.ep=!0;const r=n(i);fetch(i.href,r)}})();const F=[{path:"/",component:"List",title:"ecommerce-checkout"},{path:"/ecommerce-checkout",component:"List",title:"ecommerce-checkout"},{path:"/ecommerce-checkout/new",component:"Form",title:"New ecommerce-checkout"},{path:"/ecommerce-checkout/:id",component:"Detail",title:"ecommerce-checkout Detail"},{path:"/admin",component:"AdminDashboard",title:"Admin Dashboard"},{path:"/admin/instances",component:"AdminInstances",title:"Instances"},{path:"/admin/instances/:id",component:"AdminInstance",title:"Instance Detail"}];let w=null,v={};function P(e){e=e||"/",e!=="/"&&e.endsWith("/")&&(e=e.slice(0,-1));for(const t of F){const n={};let a=t.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");a=a.replace(/:[^/]+/g,"([^/]+)");const i=new RegExp(`^${a}$`),r=e.match(i);if(r)return(t.path.match(/:[^/]+/g)||[]).map(h=>h.slice(1)).forEach((h,J)=>{n[h]=decodeURIComponent(r[J+1])}),{route:t,params:n}}return null}function c(e,t={}){e.startsWith("/")||(e="/"+e);const n=P(e);if(!n){console.warn(`No route found for path: ${e}, falling back to list`),e="/ecommerce-checkout";const a=P(e);a&&(w=a.route,v=a.params,window.history.pushState(t,"",e),R());return}if(n.route.roles&&n.route.roles.length>0){const a=W();if(!a||!U(a,n.route.roles)){console.warn("Access denied:",e),c("/ecommerce-checkout");return}}w=n.route,v=n.params,window.history.pushState(t,"",e),R()}window.addEventListener("popstate",()=>{const e=window.location.pathname,t=P(e);t?(w=t.route,v=t.params,R()):c("/ecommerce-checkout")});function W(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function U(e,t){return!e||!e.roles?!1:t.some(n=>e.roles.includes(n))}function R(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:w,params:v}}))}function z(){return v}function E(){return w}function V(){const e=window.location.pathname,t=P(e);t?(w=t.route,v=t.params):(w=F.find(n=>n.path==="/ecommerce-checkout")||F[0],v={})}const C={brand:"ecommerce-checkout",items:[{label:"ecommerce-checkout",path:"/ecommerce-checkout",icon:""},{label:"New",path:"/ecommerce-checkout/new",icon:"+"},{label:"Admin",path:"/admin",icon:""}]};let o=null,N=!1;async function D(){if(!N){N=!0;try{const e={},t=q();t&&(e.Authorization=`Bearer ${t}`);const n=await fetch("/api/navigation",{headers:e});n.ok?o=await n.json():o=C}catch{o=C}finally{N=!1}}}async function j(){o||await D();const e=window.location.pathname,t=K(),n=(o==null?void 0:o.items)||C.items;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="/ecommerce-checkout" onclick="handleNavClick(event, '/ecommerce-checkout')">
          ${(o==null?void 0:o.brand)||C.brand}
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
  `}window.handleNavClick=function(e,t){e.preventDefault(),c(t)};window.handleLogout=async function(){try{const e=q();e&&await fetch("/auth/logout",{method:"POST",headers:{Authorization:`Bearer ${e}`}})}catch(e){console.error("Logout error:",e)}localStorage.removeItem("auth"),o=null,window.dispatchEvent(new CustomEvent("auth-change")),await L(),c("/ecommerce-checkout")};function K(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function q(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).token}catch{return null}return null}async function L(){o=null,await D();const e=document.getElementById("nav");e&&(e.innerHTML=await j())}window.addEventListener("auth-change",async()=>{await L()});window.addEventListener("route-change",()=>{const e=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(t=>{t.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(t=>{const n=t.getAttribute("href");(n===e||n!=="/"&&e.startsWith(n))&&t.parentElement.classList.add("active")})});let O=[];async function G(){try{const e=await fetch("/api/views");return e.ok?(O=await e.json(),O):(console.warn("Failed to load view definitions, using defaults"),[])}catch(e){return console.error("Error loading views:",e),[]}}const u="";let m=null,g=null,y=[],s=null;function Q(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);if(t.expires_at&&new Date(t.expires_at)>new Date)return g=t.token,m=t.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function A(e){localStorage.setItem("auth",JSON.stringify(e)),g=e.token,m=e.user,window.dispatchEvent(new CustomEvent("auth-change"))}function $(){localStorage.removeItem("auth"),g=null,m=null,window.dispatchEvent(new CustomEvent("auth-change"))}function p(){const e={"Content-Type":"application/json"};return g&&(e.Authorization=`Bearer ${g}`),e}async function b(e){if(e.status===401)throw $(),k("Session expired. Please log in again."),new Error("Unauthorized");if(!e.ok){const t=await e.json().catch(()=>({}));throw new Error(t.message||e.statusText)}return e.json()}const l={async getMe(){const e=await fetch(`${u}/auth/me`,{headers:p()});return b(e)},async logout(){await fetch(`${u}/auth/logout`,{method:"POST",headers:p()}),$()},async listInstances(){const e=await fetch(`${u}/admin/instances`,{headers:p()});return b(e)},async getInstance(e){const t=await fetch(`${u}/api/ecommercecheckout/${e}`,{headers:p()});return b(t)},async createInstance(e={}){const t=await fetch(`${u}/api/ecommercecheckout`,{method:"POST",headers:p(),body:JSON.stringify(e)});return b(t)},async executeTransition(e,t,n={}){const a=await fetch(`${u}/api/${e}`,{method:"POST",headers:p(),body:JSON.stringify({aggregate_id:t,data:n})});return b(a)}};window.api=l;window.setAuthToken=function(e){g=e};window.saveAuth=A;window.clearAuth=$;function k(e){const t=document.getElementById("app"),n=t.querySelector(".alert-error");n&&n.remove();const a=document.createElement("div");a.className="alert alert-error",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),5e3)}function M(e){const t=document.getElementById("app"),n=t.querySelector(".alert-success");n&&n.remove();const a=document.createElement("div");a.className="alert alert-success",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),3e3)}function x(e){if(!e)return"unknown";for(const[t,n]of Object.entries(e))if(n>0)return t;return"unknown"}function I(e){return`<span class="badge ${`badge-${e.toLowerCase().replace(/_/g,"-")}`}">${e.replace(/_/g," ")}</span>`}async function T(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>ecommerce-checkout</h1>
        <button class="btn btn-primary" onclick="handleCreateNew()">+ New</button>
      </div>
      <div id="instances-list" class="entity-list">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{y=(await l.listInstances()).instances||[],X()}catch{document.getElementById("instances-list").innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `}}function X(){const e=document.getElementById("instances-list");if(e){if(y.length===0){e.innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `;return}e.innerHTML=y.map(t=>{const n=x(t.state||t.places);return`
      <div class="entity-card" onclick="navigate('/ecommerce-checkout/${t.id}')">
        <div class="entity-info">
          <h3>${t.id}</h3>
          <div class="entity-meta">
            ${I(n)} &middot; Version ${t.version||0}
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
  `;try{const a=await l.getInstance(t);s={id:a.aggregate_id||t,version:a.version,state:a.state,places:a.places,enabled:a.enabled||a.enabled_transitions||[]},_()}catch(a){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${a.message}</div>
    `}}function _(){const e=document.getElementById("instance-detail");if(!e||!s)return;const t=x(s.places),n=s.enabled||[],a=[{id:"start_checkout",name:"Start Checkout",description:"Begin checkout process"},{id:"enter_payment",name:"Enter Payment",description:"Enter payment details"},{id:"process_payment",name:"Process Payment",description:"Process the payment"},{id:"payment_success",name:"Payment Success",description:"Payment processed successfully"},{id:"payment_fail_1",name:"Payment Fail 1",description:"First payment attempt failed"},{id:"retry_payment_1",name:"Retry Payment 1",description:"Retry payment (attempt 2)"},{id:"payment_fail_2",name:"Payment Fail 2",description:"Second payment attempt failed"},{id:"retry_payment_2",name:"Retry Payment 2",description:"Retry payment (attempt 3)"},{id:"payment_fail_3",name:"Payment Fail 3",description:"Third payment attempt failed"},{id:"cancel_order",name:"Cancel Order",description:"Cancel order after max retries"},{id:"fulfill",name:"Fulfill",description:"Fulfill the order"}];e.innerHTML=`
    <div class="card">
      <div class="card-header">Status</div>
      <div class="detail-list">
        <div class="detail-field">
          <dt>ID</dt>
          <dd><code>${s.id}</code></dd>
        </div>
        <div class="detail-field">
          <dt>Status</dt>
          <dd>${I(t)}</dd>
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
        ${a.map(i=>{const r=n.includes(i.id);return`
            <button
              class="btn ${r?"btn-primary":"btn-secondary"}"
              onclick="handleTransition('${i.id}')"
              ${r?"":"disabled"}
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
        ${Object.entries(s.places||{}).map(([i,r])=>`
          <div class="detail-field">
            <dt>${i}</dt>
            <dd>${r>0?`<span class="badge badge-${i}">${r} token${r>1?"s":""}</span>`:'<span style="color: #999;">0</span>'}</dd>
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
  `;try{const[t,n]=await Promise.all([fetch(`${u}/admin/stats`,{headers:p()}).then(i=>i.json()).catch(()=>null),l.listInstances()]);t?document.getElementById("admin-stats").innerHTML=`
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
      `:document.getElementById("admin-stats").innerHTML="",y=n.instances||[];const a=document.getElementById("admin-instances").querySelector(".loading");a&&(a.outerHTML=y.length>0?`<table class="table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Status</th>
                <th>Version</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              ${y.slice(0,20).map(i=>{const r=x(i.state||i.places);return`
                  <tr>
                    <td><code>${i.id}</code></td>
                    <td>${I(r)}</td>
                    <td>${i.version||0}</td>
                    <td><button class="btn btn-sm btn-link" onclick="navigate('/ecommerce-checkout/${i.id}')">View</button></td>
                  </tr>
                `}).join("")}
            </tbody>
          </table>`:'<p style="color: #666; padding: 1rem;">No instances yet.</p>')}catch(t){k("Failed to load admin data: "+t.message)}}window.navigate=c;window.handleCreateNew=async function(){c("/ecommerce-checkout/new")};window.handleSubmitCreate=async function(e){e.preventDefault();try{const t=await l.createInstance({});M("Instance created successfully!"),c(`/ecommerce-checkout/${t.aggregate_id||t.id}`)}catch(t){k("Failed to create: "+t.message)}};window.handleTransition=async function(e){if(s)try{const t=await l.executeTransition(e,s.id);s={...s,version:t.version,state:t.state,places:t.state,enabled:t.enabled||[]},_(),M(`Action "${e}" completed!`)}catch(t){k(`Failed to execute ${e}: ${t.message}`)}};function B(e){var a;const t=((a=e.detail)==null?void 0:a.route)||E();if(!t){T();return}const n=t.path;n==="/ecommerce-checkout"||n==="/"?T():n==="/ecommerce-checkout/new"?Z():n==="/ecommerce-checkout/:id"?Y():n==="/admin"||n.startsWith("/admin")?ee():T()}async function te(){const e=new URLSearchParams(window.location.search),t=e.get("token"),n=e.get("expires_at");if(t){g=t;try{const a=await l.getMe();A({token:t,expires_at:n,user:a}),window.history.replaceState({},"",window.location.pathname),await L()}catch{$(),k("Failed to complete login")}}}async function ne(){Q(),await te(),await G();const e=document.getElementById("nav");e.innerHTML=await j(),window.addEventListener("route-change",B),V(),B({detail:{route:E()}})}let f=null,S=null;function H(){const t=`${window.location.protocol==="https:"?"wss:":"ws:"}//${window.location.host}/ws`;f=new WebSocket(t),f.onopen=()=>{console.log("[Debug] WebSocket connected")},f.onmessage=n=>{try{const a=JSON.parse(n.data);a.id==="session"&&a.type==="session"?(S=(typeof a.data=="string"?JSON.parse(a.data):a.data).session_id,console.log("[Debug] Session ID:",S)):a.type==="eval"&&ae(a)}catch(a){console.error("[Debug] Failed to parse message:",a)}},f.onclose=()=>{console.log("[Debug] WebSocket disconnected, reconnecting in 3s..."),S=null,setTimeout(H,3e3)},f.onerror=n=>{console.error("[Debug] WebSocket error:",n)}}async function ae(e){try{const n=(typeof e.data=="string"?JSON.parse(e.data):e.data).code,i=await new Function("return (async () => { "+n+" })()")(),r={type:"response",id:e.id,data:{result:i,type:typeof i}};f.send(JSON.stringify(r))}catch(t){const n={type:"response",id:e.id,data:{error:t.message}};f.send(JSON.stringify(n))}}window.debugSessionId=()=>S;window.debugWs=()=>f;window.pilot={list(){return c("/ecommerce-checkout"),this.waitForRender()},newForm(){return c("/ecommerce-checkout/new"),this.waitForRender()},async view(e){return c(`/ecommerce-checkout/${e}`),await this.waitForRender(),s},admin(){return c("/admin"),this.waitForRender()},async create(e={}){const t=await l.createInstance(e),n=t.aggregate_id||t.id;return c(`/ecommerce-checkout/${n}`),await this.waitForRender(),{id:n,...t}},getCurrentInstance(){return s},getInstances(){return y},async refresh(){if(!s)throw new Error("No current instance");const e=await l.getInstance(s.id);return s={id:e.aggregate_id||s.id,version:e.version,state:e.state,places:e.places,enabled:e.enabled||e.enabled_transitions||[]},_(),s},async action(e,t={}){if(!s)throw new Error("No current instance - navigate to detail page first");const n=await l.executeTransition(e,s.id,t);return s={...s,version:n.version,state:n.state,places:n.state,enabled:n.enabled||[]},_(),{success:!0,state:s.places,enabled:s.enabled}},isEnabled(e){return s?(s.enabled||[]).includes(e):!1},getEnabled(){return(s==null?void 0:s.enabled)||[]},fill(e,t){const n=document.querySelector(`[name="${e}"]`);if(!n)throw new Error(`No input found with name: ${e}`);return n.value=t,n.dispatchEvent(new Event("input",{bubbles:!0})),this},async submit(){const e=document.querySelector("form");if(!e)throw new Error("No form found on page");const t=new Event("submit",{bubbles:!0,cancelable:!0});return e.dispatchEvent(t),await this.waitForRender(),s},getText(e){const t=document.querySelector(e);return t?t.textContent.trim():null},exists(e){return document.querySelector(e)!==null},getButtons(){return Array.from(document.querySelectorAll("button")).map(e=>({text:e.textContent.trim(),disabled:e.disabled,className:e.className}))},async clickButton(e){const t=document.querySelectorAll("button");for(const n of t)if(n.textContent.trim()===e&&!n.disabled)return n.click(),await this.waitForRender(),!0;throw new Error(`No enabled button found with text: ${e}`)},getState(){return(s==null?void 0:s.places)||null},getStatus(){if(!(s!=null&&s.places))return null;for(const[e,t]of Object.entries(s.places))if(t>0)return e;return null},getRoute(){return E()},getUser(){return m},isAuthenticated(){return g!==null},waitForRender(e=50){return new Promise(t=>setTimeout(t,e))},async waitFor(e,t=5e3){const n=Date.now();for(;Date.now()-n<t;){if(document.querySelector(e))return document.querySelector(e);await this.waitForRender(50)}throw new Error(`Timeout waiting for: ${e}`)},async waitForState(e,t=5e3){var a;const n=Date.now();for(;Date.now()-n<t;){if(((a=s==null?void 0:s.places)==null?void 0:a[e])>0)return s;await this.waitForRender(100)}throw new Error(`Timeout waiting for state: ${e}`)},debug(){return console.log("=== Pilot Debug ==="),console.log("Route:",E()),console.log("User:",m),console.log("Instance:",s),console.log("Enabled:",s==null?void 0:s.enabled),console.log("State:",s==null?void 0:s.places),{route:E(),user:m,instance:s}},async getEvents(){if(!s)throw new Error("No current instance");const e=await fetch(`${u}/api/ecommercecheckout/${s.id}/events`,{headers:p()});return(await b(e)).events||[]},async getEventCount(){return(await this.getEvents()).length},async getLastEvent(){const e=await this.getEvents();return e.length>0?e[e.length-1]:null},async replayTo(e){if(!s)throw new Error("No current instance");const n=(await this.getEvents()).filter(i=>(i.version||i.sequence)<=e),a={};for(const i of n)i.state&&Object.assign(a,i.state);return{version:e,events:n,places:a}},async loginAs(e){const t=typeof e=="string"?[e]:e,a=await(await fetch(`${u}/api/debug/login`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({login:"pilot-user",roles:t})})).json();return A(a),await this.waitForRender(100),a},logout(){return $(),this.waitForRender()},getRoles(){return(m==null?void 0:m.roles)||[]},hasRole(e){return this.getRoles().includes(e)},assertState(e){const t=this.getStatus();if(t!==e)throw new Error(`Expected state '${e}', got '${t}'`);return this},assertEnabled(e){if(!this.isEnabled(e)){const t=this.getEnabled();throw new Error(`Expected '${e}' to be enabled. Enabled: [${t.join(", ")}]`)}return this},assertDisabled(e){if(this.isEnabled(e))throw new Error(`Expected '${e}' to be disabled, but it is enabled`);return this},assertExists(e){if(!this.exists(e))throw new Error(`Expected element '${e}' to exist`);return this},assertText(e,t){const n=this.getText(e);if(n!==t)throw new Error(`Expected '${e}' to contain '${t}', got '${n}'`);return this},assertAuthenticated(){if(!this.isAuthenticated())throw new Error("Expected user to be authenticated");return this},assertRole(e){if(!this.hasRole(e))throw new Error(`Expected user to have role '${e}'. Has: [${this.getRoles().join(", ")}]`);return this},getTransitions(){return[{id:"start_checkout",name:"Start Checkout",description:"Begin checkout process"},{id:"enter_payment",name:"Enter Payment",description:"Enter payment details"},{id:"process_payment",name:"Process Payment",description:"Process the payment"},{id:"payment_success",name:"Payment Success",description:"Payment processed successfully"},{id:"payment_fail_1",name:"Payment Fail 1",description:"First payment attempt failed"},{id:"retry_payment_1",name:"Retry Payment 1",description:"Retry payment (attempt 2)"},{id:"payment_fail_2",name:"Payment Fail 2",description:"Second payment attempt failed"},{id:"retry_payment_2",name:"Retry Payment 2",description:"Retry payment (attempt 3)"},{id:"payment_fail_3",name:"Payment Fail 3",description:"Third payment attempt failed"},{id:"cancel_order",name:"Cancel Order",description:"Cancel order after max retries"},{id:"fulfill",name:"Fulfill",description:"Fulfill the order"}]},getPlaces(){return[{id:"cart",name:"Cart",initial:1},{id:"checkout_started",name:"CheckoutStarted",initial:0},{id:"payment_pending",name:"PaymentPending",initial:0},{id:"payment_processing",name:"PaymentProcessing",initial:0},{id:"retry_1",name:"Retry1",initial:0},{id:"retry_2",name:"Retry2",initial:0},{id:"retry_3",name:"Retry3",initial:0},{id:"paid",name:"Paid",initial:0},{id:"cancelled",name:"Cancelled",initial:0},{id:"fulfilled",name:"Fulfilled",initial:0}]},getTransition(e){return this.getTransitions().find(t=>t.id===e)||null},canFire(e){if(!this.getTransition(e))return{canFire:!1,reason:`Unknown transition: ${e}`};if(!s)return{canFire:!1,reason:"No current instance"};if(!this.isEnabled(e)){const a=this.getStatus();return{canFire:!1,reason:`Transition '${e}' not enabled in state '${a}'`,currentState:a,enabledTransitions:this.getEnabled()}}return{canFire:!0}},async sequence(e,t={}){const n=[],{stopOnError:a=!0,data:i={}}=t;for(const r of e){const d=this.canFire(r);if(!d.canFire){if(a)throw new Error(`Sequence failed at '${r}': ${d.reason}`);n.push({transition:r,success:!1,error:d.reason});continue}try{const h=await this.action(r,i[r]||{});n.push({transition:r,success:!0,state:h.state})}catch(h){if(a)throw h;n.push({transition:r,success:!1,error:h.message})}}return n},getWorkflowInfo(){var e;return{places:this.getPlaces(),transitions:this.getTransitions(),initialPlace:(e=this.getPlaces().find(t=>t.initial>0))==null?void 0:e.id}}};ne();H();
