(function(){const e=document.createElement("link").relList;if(e&&e.supports&&e.supports("modulepreload"))return;for(const o of document.querySelectorAll('link[rel="modulepreload"]'))a(o);new MutationObserver(o=>{for(const i of o)if(i.type==="childList")for(const d of i.addedNodes)d.tagName==="LINK"&&d.rel==="modulepreload"&&a(d)}).observe(document,{childList:!0,subtree:!0});function n(o){const i={};return o.integrity&&(i.integrity=o.integrity),o.referrerPolicy&&(i.referrerPolicy=o.referrerPolicy),o.crossOrigin==="use-credentials"?i.credentials="include":o.crossOrigin==="anonymous"?i.credentials="omit":i.credentials="same-origin",i}function a(o){if(o.ep)return;o.ep=!0;const i=n(o);fetch(o.href,i)}})();const R=[{path:"/",component:"List",title:"blog-post"},{path:"/blog-post",component:"List",title:"blog-post"},{path:"/blog-post/new",component:"Form",title:"New blog-post"},{path:"/blog-post/:id",component:"Detail",title:"blog-post Detail"},{path:"/admin",component:"AdminDashboard",title:"Admin Dashboard"},{path:"/admin/instances",component:"AdminInstances",title:"Instances"},{path:"/admin/instances/:id",component:"AdminInstance",title:"Instance Detail"}];let b=null,v={};function T(t){t=t||"/",t!=="/"&&t.endsWith("/")&&(t=t.slice(0,-1));for(const e of R){const n={};let a=e.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");a=a.replace(/:[^/]+/g,"([^/]+)");const o=new RegExp(`^${a}$`),i=t.match(o);if(i)return(e.path.match(/:[^/]+/g)||[]).map(p=>p.slice(1)).forEach((p,J)=>{n[p]=decodeURIComponent(i[J+1])}),{route:e,params:n}}return null}function c(t,e={}){t.startsWith("/")||(t="/"+t);const n=T(t);if(!n){console.warn(`No route found for path: ${t}, falling back to list`),t="/blog-post";const a=T(t);a&&(b=a.route,v=a.params,window.history.pushState(e,"",t),I());return}if(n.route.roles&&n.route.roles.length>0){const a=W();if(!a||!U(a,n.route.roles)){console.warn("Access denied:",t),c("/blog-post");return}}b=n.route,v=n.params,window.history.pushState(e,"",t),I()}window.addEventListener("popstate",()=>{const t=window.location.pathname,e=T(t);e?(b=e.route,v=e.params,I()):c("/blog-post")});function W(){const t=localStorage.getItem("auth");if(t)try{return JSON.parse(t).user}catch{return null}return null}function U(t,e){return!t||!t.roles?!1:e.some(n=>t.roles.includes(n))}function I(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:b,params:v}}))}function z(){return v}function $(){return b}function V(){const t=window.location.pathname,e=T(t);e?(b=e.route,v=e.params):(b=R.find(n=>n.path==="/blog-post")||R[0],v={})}const k={brand:"blog-post",items:[{label:"blog-post",path:"/blog-post",icon:""},{label:"New",path:"/blog-post/new",icon:"+"},{label:"Admin",path:"/admin",icon:""}]};let r=null,C=!1;async function B(){if(!C){C=!0;try{const t={},e=q();e&&(t.Authorization=`Bearer ${e}`);const n=await fetch("/api/navigation",{headers:t});n.ok?r=await n.json():r=k}catch{r=k}finally{C=!1}}}async function _(){r||await B();const t=window.location.pathname,e=K(),n=(r==null?void 0:r.items)||k.items;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="/blog-post" onclick="handleNavClick(event, '/blog-post')">
          ${(r==null?void 0:r.brand)||k.brand}
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
  `}window.handleNavClick=function(t,e){t.preventDefault(),c(e)};window.handleLogout=async function(){try{const t=q();t&&await fetch("/auth/logout",{method:"POST",headers:{Authorization:`Bearer ${t}`}})}catch(t){console.error("Logout error:",t)}localStorage.removeItem("auth"),r=null,window.dispatchEvent(new CustomEvent("auth-change")),await x(),c("/blog-post")};function K(){const t=localStorage.getItem("auth");if(t)try{return JSON.parse(t).user}catch{return null}return null}function q(){const t=localStorage.getItem("auth");if(t)try{return JSON.parse(t).token}catch{return null}return null}async function x(){r=null,await B();const t=document.getElementById("nav");t&&(t.innerHTML=await _())}window.addEventListener("auth-change",async()=>{await x()});window.addEventListener("route-change",()=>{const t=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(e=>{e.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(e=>{const n=e.getAttribute("href");(n===t||n!=="/"&&t.startsWith(n))&&e.parentElement.classList.add("active")})});let D=[];async function G(){try{const t=await fetch("/api/views");return t.ok?(D=await t.json(),D):(console.warn("Failed to load view definitions, using defaults"),[])}catch(t){return console.error("Error loading views:",t),[]}}const u="";let h=null,f=null,w=[],s=null;function Q(){const t=localStorage.getItem("auth");if(t)try{const e=JSON.parse(t);if(e.expires_at&&new Date(e.expires_at)>new Date)return f=e.token,h=e.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function F(t){localStorage.setItem("auth",JSON.stringify(t)),f=t.token,h=t.user,window.dispatchEvent(new CustomEvent("auth-change"))}function E(){localStorage.removeItem("auth"),f=null,h=null,window.dispatchEvent(new CustomEvent("auth-change"))}function g(){const t={"Content-Type":"application/json"};return f&&(t.Authorization=`Bearer ${f}`),t}async function y(t){if(t.status===401)throw E(),S("Session expired. Please log in again."),new Error("Unauthorized");if(!t.ok){const e=await t.json().catch(()=>({}));throw new Error(e.message||t.statusText)}return t.json()}const l={async getMe(){const t=await fetch(`${u}/auth/me`,{headers:g()});return y(t)},async logout(){await fetch(`${u}/auth/logout`,{method:"POST",headers:g()}),E()},async listInstances(){const t=await fetch(`${u}/admin/instances`,{headers:g()});return y(t)},async getInstance(t){const e=await fetch(`${u}/api/blogpost/${t}`,{headers:g()});return y(e)},async createInstance(t={}){const e=await fetch(`${u}/api/blogpost`,{method:"POST",headers:g(),body:JSON.stringify(t)});return y(e)},async executeTransition(t,e,n={}){const a=await fetch(`${u}/api/${t}`,{method:"POST",headers:g(),body:JSON.stringify({aggregate_id:e,data:n})});return y(a)}};window.api=l;window.setAuthToken=function(t){f=t};window.saveAuth=F;window.clearAuth=E;function S(t){const e=document.getElementById("app"),n=e.querySelector(".alert-error");n&&n.remove();const a=document.createElement("div");a.className="alert alert-error",a.textContent=t,e.insertBefore(a,e.firstChild),setTimeout(()=>a.remove(),5e3)}function M(t){const e=document.getElementById("app"),n=e.querySelector(".alert-success");n&&n.remove();const a=document.createElement("div");a.className="alert alert-success",a.textContent=t,e.insertBefore(a,e.firstChild),setTimeout(()=>a.remove(),3e3)}function O(t){if(!t)return"unknown";for(const[e,n]of Object.entries(t))if(n>0)return e;return"unknown"}function P(t){return`<span class="badge ${`badge-${t.toLowerCase().replace(/_/g,"-")}`}">${t.replace(/_/g," ")}</span>`}async function L(){const t=document.getElementById("app");t.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>blog-post</h1>
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
    `}}function X(){const t=document.getElementById("instances-list");if(t){if(w.length===0){t.innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `;return}t.innerHTML=w.map(e=>{const n=O(e.state||e.places);return`
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
  `;try{const a=await l.getInstance(e);s={id:a.aggregate_id||e,version:a.version,state:a.state,places:a.places,enabled:a.enabled||a.enabled_transitions||[]},A()}catch(a){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${a.message}</div>
    `}}function A(){const t=document.getElementById("instance-detail");if(!t||!s)return;const e=O(s.places),n=s.enabled||[],a=[{id:"submit",name:"Submit",description:"Submit draft for review"},{id:"approve",name:"Approve",description:"Approve and publish the post"},{id:"reject",name:"Reject",description:"Reject and return to draft"},{id:"unpublish",name:"Unpublish",description:"Take down a published post"},{id:"restore",name:"Restore",description:"Restore archived post to draft"}];t.innerHTML=`
    <div class="card">
      <div class="card-header">Status</div>
      <div class="detail-list">
        <div class="detail-field">
          <dt>ID</dt>
          <dd><code>${s.id}</code></dd>
        </div>
        <div class="detail-field">
          <dt>Status</dt>
          <dd>${P(e)}</dd>
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
        ${a.map(o=>{const i=n.includes(o.id);return`
            <button
              class="btn ${i?"btn-primary":"btn-secondary"}"
              onclick="handleTransition('${o.id}')"
              ${i?"":"disabled"}
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
        ${Object.entries(s.places||{}).map(([o,i])=>`
          <div class="detail-field">
            <dt>${o}</dt>
            <dd>${i>0?`<span class="badge badge-${o}">${i} token${i>1?"s":""}</span>`:'<span style="color: #999;">0</span>'}</dd>
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
  `;try{const[e,n]=await Promise.all([fetch(`${u}/admin/stats`,{headers:g()}).then(o=>o.json()).catch(()=>null),l.listInstances()]);e?document.getElementById("admin-stats").innerHTML=`
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
              ${w.slice(0,20).map(o=>{const i=O(o.state||o.places);return`
                  <tr>
                    <td><code>${o.id}</code></td>
                    <td>${P(i)}</td>
                    <td>${o.version||0}</td>
                    <td><button class="btn btn-sm btn-link" onclick="navigate('/blog-post/${o.id}')">View</button></td>
                  </tr>
                `}).join("")}
            </tbody>
          </table>`:'<p style="color: #666; padding: 1rem;">No instances yet.</p>')}catch(e){S("Failed to load admin data: "+e.message)}}window.navigate=c;window.handleCreateNew=async function(){c("/blog-post/new")};window.handleSubmitCreate=async function(t){t.preventDefault();try{const e=await l.createInstance({});M("Instance created successfully!"),c(`/blog-post/${e.aggregate_id||e.id}`)}catch(e){S("Failed to create: "+e.message)}};window.handleTransition=async function(t){if(s)try{const e=await l.executeTransition(t,s.id);s={...s,version:e.version,state:e.state,places:e.state,enabled:e.enabled||[]},A(),M(`Action "${t}" completed!`)}catch(e){S(`Failed to execute ${t}: ${e.message}`)}};function j(t){var a;const e=((a=t.detail)==null?void 0:a.route)||$();if(!e){L();return}const n=e.path;n==="/blog-post"||n==="/"?L():n==="/blog-post/new"?Z():n==="/blog-post/:id"?Y():n==="/admin"||n.startsWith("/admin")?tt():L()}async function et(){const t=new URLSearchParams(window.location.search),e=t.get("token"),n=t.get("expires_at");if(e){f=e;try{const a=await l.getMe();F({token:e,expires_at:n,user:a}),window.history.replaceState({},"",window.location.pathname),await x()}catch{E(),S("Failed to complete login")}}}async function nt(){Q(),await et(),await G();const t=document.getElementById("nav");t.innerHTML=await _(),window.addEventListener("route-change",j),V(),j({detail:{route:$()}})}let m=null,N=null;function H(){const e=`${window.location.protocol==="https:"?"wss:":"ws:"}//${window.location.host}/ws`;m=new WebSocket(e),m.onopen=()=>{console.log("[Debug] WebSocket connected")},m.onmessage=n=>{try{const a=JSON.parse(n.data);a.id==="session"&&a.type==="session"?(N=(typeof a.data=="string"?JSON.parse(a.data):a.data).session_id,console.log("[Debug] Session ID:",N)):a.type==="eval"&&at(a)}catch(a){console.error("[Debug] Failed to parse message:",a)}},m.onclose=()=>{console.log("[Debug] WebSocket disconnected, reconnecting in 3s..."),N=null,setTimeout(H,3e3)},m.onerror=n=>{console.error("[Debug] WebSocket error:",n)}}async function at(t){try{const n=(typeof t.data=="string"?JSON.parse(t.data):t.data).code,o=await new Function("return (async () => { "+n+" })()")(),i={type:"response",id:t.id,data:{result:o,type:typeof o}};m.send(JSON.stringify(i))}catch(e){const n={type:"response",id:t.id,data:{error:e.message}};m.send(JSON.stringify(n))}}window.debugSessionId=()=>N;window.debugWs=()=>m;window.pilot={list(){return c("/blog-post"),this.waitForRender()},newForm(){return c("/blog-post/new"),this.waitForRender()},async view(t){return c(`/blog-post/${t}`),await this.waitForRender(),s},admin(){return c("/admin"),this.waitForRender()},async create(t={}){const e=await l.createInstance(t),n=e.aggregate_id||e.id;return c(`/blog-post/${n}`),await this.waitForRender(),{id:n,...e}},getCurrentInstance(){return s},getInstances(){return w},async refresh(){if(!s)throw new Error("No current instance");const t=await l.getInstance(s.id);return s={id:t.aggregate_id||s.id,version:t.version,state:t.state,places:t.places,enabled:t.enabled||t.enabled_transitions||[]},A(),s},async action(t,e={}){if(!s)throw new Error("No current instance - navigate to detail page first");const n=await l.executeTransition(t,s.id,e);return s={...s,version:n.version,state:n.state,places:n.state,enabled:n.enabled||[]},A(),{success:!0,state:s.places,enabled:s.enabled}},isEnabled(t){return s?(s.enabled||[]).includes(t):!1},getEnabled(){return(s==null?void 0:s.enabled)||[]},fill(t,e){const n=document.querySelector(`[name="${t}"]`);if(!n)throw new Error(`No input found with name: ${t}`);return n.value=e,n.dispatchEvent(new Event("input",{bubbles:!0})),this},async submit(){const t=document.querySelector("form");if(!t)throw new Error("No form found on page");const e=new Event("submit",{bubbles:!0,cancelable:!0});return t.dispatchEvent(e),await this.waitForRender(),s},getText(t){const e=document.querySelector(t);return e?e.textContent.trim():null},exists(t){return document.querySelector(t)!==null},getButtons(){return Array.from(document.querySelectorAll("button")).map(t=>({text:t.textContent.trim(),disabled:t.disabled,className:t.className}))},async clickButton(t){const e=document.querySelectorAll("button");for(const n of e)if(n.textContent.trim()===t&&!n.disabled)return n.click(),await this.waitForRender(),!0;throw new Error(`No enabled button found with text: ${t}`)},getState(){return(s==null?void 0:s.places)||null},getStatus(){if(!(s!=null&&s.places))return null;for(const[t,e]of Object.entries(s.places))if(e>0)return t;return null},getRoute(){return $()},getUser(){return h},isAuthenticated(){return f!==null},waitForRender(t=50){return new Promise(e=>setTimeout(e,t))},async waitFor(t,e=5e3){const n=Date.now();for(;Date.now()-n<e;){if(document.querySelector(t))return document.querySelector(t);await this.waitForRender(50)}throw new Error(`Timeout waiting for: ${t}`)},async waitForState(t,e=5e3){var a;const n=Date.now();for(;Date.now()-n<e;){if(((a=s==null?void 0:s.places)==null?void 0:a[t])>0)return s;await this.waitForRender(100)}throw new Error(`Timeout waiting for state: ${t}`)},debug(){return console.log("=== Pilot Debug ==="),console.log("Route:",$()),console.log("User:",h),console.log("Instance:",s),console.log("Enabled:",s==null?void 0:s.enabled),console.log("State:",s==null?void 0:s.places),{route:$(),user:h,instance:s}},async getEvents(){if(!s)throw new Error("No current instance");const t=await fetch(`${u}/api/blogpost/${s.id}/events`,{headers:g()});return(await y(t)).events||[]},async getEventCount(){return(await this.getEvents()).length},async getLastEvent(){const t=await this.getEvents();return t.length>0?t[t.length-1]:null},async replayTo(t){if(!s)throw new Error("No current instance");const n=(await this.getEvents()).filter(o=>(o.version||o.sequence)<=t),a={};for(const o of n)o.state&&Object.assign(a,o.state);return{version:t,events:n,places:a}},async loginAs(t){const e=typeof t=="string"?[t]:t,a=await(await fetch(`${u}/api/debug/login`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({login:"pilot-user",roles:e})})).json();return F(a),await this.waitForRender(100),a},logout(){return E(),this.waitForRender()},getRoles(){return(h==null?void 0:h.roles)||[]},hasRole(t){return this.getRoles().includes(t)},assertState(t){const e=this.getStatus();if(e!==t)throw new Error(`Expected state '${t}', got '${e}'`);return this},assertEnabled(t){if(!this.isEnabled(t)){const e=this.getEnabled();throw new Error(`Expected '${t}' to be enabled. Enabled: [${e.join(", ")}]`)}return this},assertDisabled(t){if(this.isEnabled(t))throw new Error(`Expected '${t}' to be disabled, but it is enabled`);return this},assertExists(t){if(!this.exists(t))throw new Error(`Expected element '${t}' to exist`);return this},assertText(t,e){const n=this.getText(t);if(n!==e)throw new Error(`Expected '${t}' to contain '${e}', got '${n}'`);return this},assertAuthenticated(){if(!this.isAuthenticated())throw new Error("Expected user to be authenticated");return this},assertRole(t){if(!this.hasRole(t))throw new Error(`Expected user to have role '${t}'. Has: [${this.getRoles().join(", ")}]`);return this},getTransitions(){return[{id:"submit",name:"Submit",description:"Submit draft for review"},{id:"approve",name:"Approve",description:"Approve and publish the post"},{id:"reject",name:"Reject",description:"Reject and return to draft"},{id:"unpublish",name:"Unpublish",description:"Take down a published post"},{id:"restore",name:"Restore",description:"Restore archived post to draft"}]},getPlaces(){return[{id:"draft",name:"Draft",initial:1},{id:"in_review",name:"InReview",initial:0},{id:"published",name:"Published",initial:0},{id:"archived",name:"Archived",initial:0}]},getTransition(t){return this.getTransitions().find(e=>e.id===t)||null},canFire(t){if(!this.getTransition(t))return{canFire:!1,reason:`Unknown transition: ${t}`};if(!s)return{canFire:!1,reason:"No current instance"};if(!this.isEnabled(t)){const a=this.getStatus();return{canFire:!1,reason:`Transition '${t}' not enabled in state '${a}'`,currentState:a,enabledTransitions:this.getEnabled()}}return{canFire:!0}},async sequence(t,e={}){const n=[],{stopOnError:a=!0,data:o={}}=e;for(const i of t){const d=this.canFire(i);if(!d.canFire){if(a)throw new Error(`Sequence failed at '${i}': ${d.reason}`);n.push({transition:i,success:!1,error:d.reason});continue}try{const p=await this.action(i,o[i]||{});n.push({transition:i,success:!0,state:p.state})}catch(p){if(a)throw p;n.push({transition:i,success:!1,error:p.message})}}return n},getWorkflowInfo(){var t;return{places:this.getPlaces(),transitions:this.getTransitions(),initialPlace:(t=this.getPlaces().find(e=>e.initial>0))==null?void 0:t.id}}};nt();H();
