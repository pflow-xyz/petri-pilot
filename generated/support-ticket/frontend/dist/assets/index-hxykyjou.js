(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const i of document.querySelectorAll('link[rel="modulepreload"]'))s(i);new MutationObserver(i=>{for(const r of i)if(r.type==="childList")for(const d of r.addedNodes)d.tagName==="LINK"&&d.rel==="modulepreload"&&s(d)}).observe(document,{childList:!0,subtree:!0});function n(i){const r={};return i.integrity&&(r.integrity=i.integrity),i.referrerPolicy&&(r.referrerPolicy=i.referrerPolicy),i.crossOrigin==="use-credentials"?r.credentials="include":i.crossOrigin==="anonymous"?r.credentials="omit":r.credentials="same-origin",r}function s(i){if(i.ep)return;i.ep=!0;const r=n(i);fetch(i.href,r)}})();const L=[{path:"/",component:"List",title:"support-ticket"},{path:"/support-ticket",component:"List",title:"support-ticket"},{path:"/support-ticket/new",component:"Form",title:"New support-ticket"},{path:"/support-ticket/:id",component:"Detail",title:"support-ticket Detail"},{path:"/admin",component:"AdminDashboard",title:"Admin Dashboard"},{path:"/admin/instances",component:"AdminInstances",title:"Instances"},{path:"/admin/instances/:id",component:"AdminInstance",title:"Instance Detail"}];let v=null,b={};function C(e){e=e||"/",e!=="/"&&e.endsWith("/")&&(e=e.slice(0,-1));for(const t of L){const n={};let s=t.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");s=s.replace(/:[^/]+/g,"([^/]+)");const i=new RegExp(`^${s}$`),r=e.match(i);if(r)return(t.path.match(/:[^/]+/g)||[]).map(h=>h.slice(1)).forEach((h,W)=>{n[h]=decodeURIComponent(r[W+1])}),{route:t,params:n}}return null}function c(e,t={}){e.startsWith("/")||(e="/"+e);const n=C(e);if(!n){console.warn(`No route found for path: ${e}, falling back to list`),e="/support-ticket";const s=C(e);s&&(v=s.route,b=s.params,window.history.pushState(t,"",e),I());return}if(n.route.roles&&n.route.roles.length>0){const s=J();if(!s||!U(s,n.route.roles)){console.warn("Access denied:",e),c("/support-ticket");return}}v=n.route,b=n.params,window.history.pushState(t,"",e),I()}window.addEventListener("popstate",()=>{const e=window.location.pathname,t=C(e);t?(v=t.route,b=t.params,I()):c("/support-ticket")});function J(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function U(e,t){return!e||!e.roles?!1:t.some(n=>e.roles.includes(n))}function I(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:v,params:b}}))}function z(){return b}function k(){return v}function V(){const e=window.location.pathname,t=C(e);t?(v=t.route,b=t.params):(v=L.find(n=>n.path==="/support-ticket")||L[0],b={})}const N={brand:"support-ticket",items:[{label:"support-ticket",path:"/support-ticket",icon:""},{label:"New",path:"/support-ticket/new",icon:"+"},{label:"Admin",path:"/admin",icon:""}]};let o=null,R=!1;async function D(){if(!R){R=!0;try{const e={},t=M();t&&(e.Authorization=`Bearer ${t}`);const n=await fetch("/api/navigation",{headers:e});n.ok?o=await n.json():o=N}catch{o=N}finally{R=!1}}}async function q(){o||await D();const e=window.location.pathname,t=K(),n=(o==null?void 0:o.items)||N.items;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="/support-ticket" onclick="handleNavClick(event, '/support-ticket')">
          ${(o==null?void 0:o.brand)||N.brand}
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
  `}window.handleNavClick=function(e,t){e.preventDefault(),c(t)};window.handleLogout=async function(){try{const e=M();e&&await fetch("/auth/logout",{method:"POST",headers:{Authorization:`Bearer ${e}`}})}catch(e){console.error("Logout error:",e)}localStorage.removeItem("auth"),o=null,window.dispatchEvent(new CustomEvent("auth-change")),await x(),c("/support-ticket")};function K(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function M(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).token}catch{return null}return null}async function x(){o=null,await D();const e=document.getElementById("nav");e&&(e.innerHTML=await q())}window.addEventListener("auth-change",async()=>{await x()});window.addEventListener("route-change",()=>{const e=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(t=>{t.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(t=>{const n=t.getAttribute("href");(n===e||n!=="/"&&e.startsWith(n))&&t.parentElement.classList.add("active")})});let _=[];async function G(){try{const e=await fetch("/api/views");return e.ok?(_=await e.json(),_):(console.warn("Failed to load view definitions, using defaults"),[])}catch(e){return console.error("Error loading views:",e),[]}}const u="";let p=null,f=null,w=[],a=null;function Q(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);if(t.expires_at&&new Date(t.expires_at)>new Date)return f=t.token,p=t.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function F(e){localStorage.setItem("auth",JSON.stringify(e)),f=e.token,p=e.user,window.dispatchEvent(new CustomEvent("auth-change"))}function E(){localStorage.removeItem("auth"),f=null,p=null,window.dispatchEvent(new CustomEvent("auth-change"))}function m(){const e={"Content-Type":"application/json"};return f&&(e.Authorization=`Bearer ${f}`),e}async function y(e){if(e.status===401)throw E(),$("Session expired. Please log in again."),new Error("Unauthorized");if(!e.ok){const t=await e.json().catch(()=>({}));throw new Error(t.message||e.statusText)}return e.json()}const l={async getMe(){const e=await fetch(`${u}/auth/me`,{headers:m()});return y(e)},async logout(){await fetch(`${u}/auth/logout`,{method:"POST",headers:m()}),E()},async listInstances(){const e=await fetch(`${u}/admin/instances`,{headers:m()});return y(e)},async getInstance(e){const t=await fetch(`${u}/api/supportticket/${e}`,{headers:m()});return y(t)},async createInstance(e={}){const t=await fetch(`${u}/api/supportticket`,{method:"POST",headers:m(),body:JSON.stringify(e)});return y(t)},async executeTransition(e,t,n={}){const s=await fetch(`${u}/api/${e}`,{method:"POST",headers:m(),body:JSON.stringify({aggregate_id:t,data:n})});return y(s)}};window.api=l;window.setAuthToken=function(e){f=e};window.saveAuth=F;window.clearAuth=E;function $(e){const t=document.getElementById("app"),n=t.querySelector(".alert-error");n&&n.remove();const s=document.createElement("div");s.className="alert alert-error",s.textContent=e,t.insertBefore(s,t.firstChild),setTimeout(()=>s.remove(),5e3)}function j(e){const t=document.getElementById("app"),n=t.querySelector(".alert-success");n&&n.remove();const s=document.createElement("div");s.className="alert alert-success",s.textContent=e,t.insertBefore(s,t.firstChild),setTimeout(()=>s.remove(),3e3)}function P(e){if(!e)return"unknown";for(const[t,n]of Object.entries(e))if(n>0)return t;return"unknown"}function O(e){return`<span class="badge ${`badge-${e.toLowerCase().replace(/_/g,"-")}`}">${e.replace(/_/g," ")}</span>`}async function A(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>support-ticket</h1>
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
    `;return}e.innerHTML=w.map(t=>{const n=P(t.state||t.places);return`
      <div class="entity-card" onclick="navigate('/support-ticket/${t.id}')">
        <div class="entity-info">
          <h3>${t.id}</h3>
          <div class="entity-meta">
            ${O(n)} &middot; Version ${t.version||0}
          </div>
        </div>
        <div class="entity-actions">
          <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); navigate('/support-ticket/${t.id}')">
            View
          </button>
        </div>
      </div>
    `}).join("")}}async function Y(){const t=z().id,n=document.getElementById("app");n.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/support-ticket')" style="margin-left: -0.5rem">
            &larr; Back to List
          </button>
          <h1 style="margin-top: 0.5rem">Instance: ${t}</h1>
        </div>
      </div>
      <div id="instance-detail">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{const s=await l.getInstance(t);a={id:s.aggregate_id||t,version:s.version,state:s.state,places:s.places,enabled:s.enabled||s.enabled_transitions||[]},T()}catch(s){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${s.message}</div>
    `}}function T(){const e=document.getElementById("instance-detail");if(!e||!a)return;const t=P(a.places),n=a.enabled||[],s=[{id:"assign",name:"Assign",description:"Assign ticket to an agent"},{id:"start_work",name:"Start Work",description:"Begin working on the ticket"},{id:"escalate",name:"Escalate",description:"Escalate to senior support"},{id:"request_info",name:"Request Info",description:"Request more information from customer"},{id:"customer_reply",name:"Customer Reply",description:"Customer provides requested information"},{id:"resolve",name:"Resolve",description:"Mark issue as resolved from in_progress"},{id:"resolve_escalated",name:"Resolve Escalated",description:"Mark escalated issue as resolved"},{id:"close",name:"Close",description:"Close the ticket"},{id:"reopen",name:"Reopen",description:"Customer reopens a closed ticket"}];e.innerHTML=`
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
        ${s.map(i=>{const r=n.includes(i.id);return`
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
        ${Object.entries(a.places||{}).map(([i,r])=>`
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
  `;try{const[t,n]=await Promise.all([fetch(`${u}/admin/stats`,{headers:m()}).then(i=>i.json()).catch(()=>null),l.listInstances()]);t?document.getElementById("admin-stats").innerHTML=`
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
              ${w.slice(0,20).map(i=>{const r=P(i.state||i.places);return`
                  <tr>
                    <td><code>${i.id}</code></td>
                    <td>${O(r)}</td>
                    <td>${i.version||0}</td>
                    <td><button class="btn btn-sm btn-link" onclick="navigate('/support-ticket/${i.id}')">View</button></td>
                  </tr>
                `}).join("")}
            </tbody>
          </table>`:'<p style="color: #666; padding: 1rem;">No instances yet.</p>')}catch(t){$("Failed to load admin data: "+t.message)}}window.navigate=c;window.handleCreateNew=async function(){c("/support-ticket/new")};window.handleSubmitCreate=async function(e){e.preventDefault();try{const t=await l.createInstance({});j("Instance created successfully!"),c(`/support-ticket/${t.aggregate_id||t.id}`)}catch(t){$("Failed to create: "+t.message)}};window.handleTransition=async function(e){if(a)try{const t=await l.executeTransition(e,a.id);a={...a,version:t.version,state:t.state,places:t.state,enabled:t.enabled||[]},T(),j(`Action "${e}" completed!`)}catch(t){$(`Failed to execute ${e}: ${t.message}`)}};function B(e){var s;const t=((s=e.detail)==null?void 0:s.route)||k();if(!t){A();return}const n=t.path;n==="/support-ticket"||n==="/"?A():n==="/support-ticket/new"?Z():n==="/support-ticket/:id"?Y():n==="/admin"||n.startsWith("/admin")?ee():A()}async function te(){const e=new URLSearchParams(window.location.search),t=e.get("token"),n=e.get("expires_at");if(t){f=t;try{const s=await l.getMe();F({token:t,expires_at:n,user:s}),window.history.replaceState({},"",window.location.pathname),await x()}catch{E(),$("Failed to complete login")}}}async function ne(){Q(),await te(),await G();const e=document.getElementById("nav");e.innerHTML=await q(),window.addEventListener("route-change",B),V(),B({detail:{route:k()}})}let g=null,S=null;function H(){const t=`${window.location.protocol==="https:"?"wss:":"ws:"}//${window.location.host}/ws`;g=new WebSocket(t),g.onopen=()=>{console.log("[Debug] WebSocket connected")},g.onmessage=n=>{try{const s=JSON.parse(n.data);s.id==="session"&&s.type==="session"?(S=(typeof s.data=="string"?JSON.parse(s.data):s.data).session_id,console.log("[Debug] Session ID:",S)):s.type==="eval"&&se(s)}catch(s){console.error("[Debug] Failed to parse message:",s)}},g.onclose=()=>{console.log("[Debug] WebSocket disconnected, reconnecting in 3s..."),S=null,setTimeout(H,3e3)},g.onerror=n=>{console.error("[Debug] WebSocket error:",n)}}async function se(e){try{const n=(typeof e.data=="string"?JSON.parse(e.data):e.data).code,i=await new Function("return (async () => { "+n+" })()")(),r={type:"response",id:e.id,data:{result:i,type:typeof i}};g.send(JSON.stringify(r))}catch(t){const n={type:"response",id:e.id,data:{error:t.message}};g.send(JSON.stringify(n))}}window.debugSessionId=()=>S;window.debugWs=()=>g;window.pilot={list(){return c("/support-ticket"),this.waitForRender()},newForm(){return c("/support-ticket/new"),this.waitForRender()},async view(e){return c(`/support-ticket/${e}`),await this.waitForRender(),a},admin(){return c("/admin"),this.waitForRender()},async create(e={}){const t=await l.createInstance(e),n=t.aggregate_id||t.id;return c(`/support-ticket/${n}`),await this.waitForRender(),{id:n,...t}},getCurrentInstance(){return a},getInstances(){return w},async refresh(){if(!a)throw new Error("No current instance");const e=await l.getInstance(a.id);return a={id:e.aggregate_id||a.id,version:e.version,state:e.state,places:e.places,enabled:e.enabled||e.enabled_transitions||[]},T(),a},async action(e,t={}){if(!a)throw new Error("No current instance - navigate to detail page first");const n=await l.executeTransition(e,a.id,t);return a={...a,version:n.version,state:n.state,places:n.state,enabled:n.enabled||[]},T(),{success:!0,state:a.places,enabled:a.enabled}},isEnabled(e){return a?(a.enabled||[]).includes(e):!1},getEnabled(){return(a==null?void 0:a.enabled)||[]},fill(e,t){const n=document.querySelector(`[name="${e}"]`);if(!n)throw new Error(`No input found with name: ${e}`);return n.value=t,n.dispatchEvent(new Event("input",{bubbles:!0})),this},async submit(){const e=document.querySelector("form");if(!e)throw new Error("No form found on page");const t=new Event("submit",{bubbles:!0,cancelable:!0});return e.dispatchEvent(t),await this.waitForRender(),a},getText(e){const t=document.querySelector(e);return t?t.textContent.trim():null},exists(e){return document.querySelector(e)!==null},getButtons(){return Array.from(document.querySelectorAll("button")).map(e=>({text:e.textContent.trim(),disabled:e.disabled,className:e.className}))},async clickButton(e){const t=document.querySelectorAll("button");for(const n of t)if(n.textContent.trim()===e&&!n.disabled)return n.click(),await this.waitForRender(),!0;throw new Error(`No enabled button found with text: ${e}`)},getState(){return(a==null?void 0:a.places)||null},getStatus(){if(!(a!=null&&a.places))return null;for(const[e,t]of Object.entries(a.places))if(t>0)return e;return null},getRoute(){return k()},getUser(){return p},isAuthenticated(){return f!==null},waitForRender(e=50){return new Promise(t=>setTimeout(t,e))},async waitFor(e,t=5e3){const n=Date.now();for(;Date.now()-n<t;){if(document.querySelector(e))return document.querySelector(e);await this.waitForRender(50)}throw new Error(`Timeout waiting for: ${e}`)},async waitForState(e,t=5e3){var s;const n=Date.now();for(;Date.now()-n<t;){if(((s=a==null?void 0:a.places)==null?void 0:s[e])>0)return a;await this.waitForRender(100)}throw new Error(`Timeout waiting for state: ${e}`)},debug(){return console.log("=== Pilot Debug ==="),console.log("Route:",k()),console.log("User:",p),console.log("Instance:",a),console.log("Enabled:",a==null?void 0:a.enabled),console.log("State:",a==null?void 0:a.places),{route:k(),user:p,instance:a}},async getEvents(){if(!a)throw new Error("No current instance");const e=await fetch(`${u}/api/supportticket/${a.id}/events`,{headers:m()});return(await y(e)).events||[]},async getEventCount(){return(await this.getEvents()).length},async getLastEvent(){const e=await this.getEvents();return e.length>0?e[e.length-1]:null},async replayTo(e){if(!a)throw new Error("No current instance");const n=(await this.getEvents()).filter(i=>(i.version||i.sequence)<=e),s={};for(const i of n)i.state&&Object.assign(s,i.state);return{version:e,events:n,places:s}},async loginAs(e){const t=typeof e=="string"?[e]:e,s=await(await fetch(`${u}/api/debug/login`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({login:"pilot-user",roles:t})})).json();return F(s),await this.waitForRender(100),s},logout(){return E(),this.waitForRender()},getRoles(){return(p==null?void 0:p.roles)||[]},hasRole(e){return this.getRoles().includes(e)},assertState(e){const t=this.getStatus();if(t!==e)throw new Error(`Expected state '${e}', got '${t}'`);return this},assertEnabled(e){if(!this.isEnabled(e)){const t=this.getEnabled();throw new Error(`Expected '${e}' to be enabled. Enabled: [${t.join(", ")}]`)}return this},assertDisabled(e){if(this.isEnabled(e))throw new Error(`Expected '${e}' to be disabled, but it is enabled`);return this},assertExists(e){if(!this.exists(e))throw new Error(`Expected element '${e}' to exist`);return this},assertText(e,t){const n=this.getText(e);if(n!==t)throw new Error(`Expected '${e}' to contain '${t}', got '${n}'`);return this},assertAuthenticated(){if(!this.isAuthenticated())throw new Error("Expected user to be authenticated");return this},assertRole(e){if(!this.hasRole(e))throw new Error(`Expected user to have role '${e}'. Has: [${this.getRoles().join(", ")}]`);return this},getTransitions(){return[{id:"assign",name:"Assign",description:"Assign ticket to an agent"},{id:"start_work",name:"Start Work",description:"Begin working on the ticket"},{id:"escalate",name:"Escalate",description:"Escalate to senior support"},{id:"request_info",name:"Request Info",description:"Request more information from customer"},{id:"customer_reply",name:"Customer Reply",description:"Customer provides requested information"},{id:"resolve",name:"Resolve",description:"Mark issue as resolved from in_progress"},{id:"resolve_escalated",name:"Resolve Escalated",description:"Mark escalated issue as resolved"},{id:"close",name:"Close",description:"Close the ticket"},{id:"reopen",name:"Reopen",description:"Customer reopens a closed ticket"}]},getPlaces(){return[{id:"new",name:"New",initial:1},{id:"assigned",name:"Assigned",initial:0},{id:"in_progress",name:"InProgress",initial:0},{id:"escalated",name:"Escalated",initial:0},{id:"pending_customer",name:"PendingCustomer",initial:0},{id:"resolved",name:"Resolved",initial:0},{id:"closed",name:"Closed",initial:0}]},getTransition(e){return this.getTransitions().find(t=>t.id===e)||null},canFire(e){if(!this.getTransition(e))return{canFire:!1,reason:`Unknown transition: ${e}`};if(!a)return{canFire:!1,reason:"No current instance"};if(!this.isEnabled(e)){const s=this.getStatus();return{canFire:!1,reason:`Transition '${e}' not enabled in state '${s}'`,currentState:s,enabledTransitions:this.getEnabled()}}return{canFire:!0}},async sequence(e,t={}){const n=[],{stopOnError:s=!0,data:i={}}=t;for(const r of e){const d=this.canFire(r);if(!d.canFire){if(s)throw new Error(`Sequence failed at '${r}': ${d.reason}`);n.push({transition:r,success:!1,error:d.reason});continue}try{const h=await this.action(r,i[r]||{});n.push({transition:r,success:!0,state:h.state})}catch(h){if(s)throw h;n.push({transition:r,success:!1,error:h.message})}}return n},getWorkflowInfo(){var e;return{places:this.getPlaces(),transitions:this.getTransitions(),initialPlace:(e=this.getPlaces().find(t=>t.initial>0))==null?void 0:e.id}}};ne();H();
