(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const o of document.querySelectorAll('link[rel="modulepreload"]'))a(o);new MutationObserver(o=>{for(const r of o)if(r.type==="childList")for(const l of r.addedNodes)l.tagName==="LINK"&&l.rel==="modulepreload"&&a(l)}).observe(document,{childList:!0,subtree:!0});function n(o){const r={};return o.integrity&&(r.integrity=o.integrity),o.referrerPolicy&&(r.referrerPolicy=o.referrerPolicy),o.crossOrigin==="use-credentials"?r.credentials="include":o.crossOrigin==="anonymous"?r.credentials="omit":r.credentials="same-origin",r}function a(o){if(o.ep)return;o.ep=!0;const r=n(o);fetch(o.href,r)}})();const K=[{path:"/",component:"List",title:"erc20-token"},{path:"/erc20token",component:"List",title:"erc20-token"},{path:"/erc20token/new",component:"Form",title:"New erc20-token"},{path:"/erc20token/:id",component:"Detail",title:"erc20-token Detail"},{path:"/admin",component:"AdminDashboard",title:"Admin Dashboard"},{path:"/admin/instances",component:"AdminInstances",title:"Instances"},{path:"/admin/instances/:id",component:"AdminInstance",title:"Instance Detail"}];let E=null,S={};function j(e){e=e||"/",e!=="/"&&e.endsWith("/")&&(e=e.slice(0,-1));for(const t of K){const n={};let a=t.path.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");a=a.replace(/:[^/]+/g,"([^/]+)");const o=new RegExp(`^${a}$`),r=e.match(o);if(r)return(t.path.match(/:[^/]+/g)||[]).map(i=>i.slice(1)).forEach((i,c)=>{n[i]=decodeURIComponent(r[c+1])}),{route:t,params:n}}return null}function m(e,t={}){e.startsWith("/")||(e="/"+e);const n=j(e);if(!n){console.warn(`No route found for path: ${e}, falling back to list`),e="/erc20token";const a=j(e);a&&(E=a.route,S=a.params,window.history.pushState(t,"",e),X());return}if(n.route.roles&&n.route.roles.length>0){const a=ve();if(!a||!ye(a,n.route.roles)){console.warn("Access denied:",e),m("/erc20token");return}}E=n.route,S=n.params,window.history.pushState(t,"",e),X()}window.addEventListener("popstate",()=>{const e=window.location.pathname,t=j(e);t?(E=t.route,S=t.params,X()):m("/erc20token")});function ve(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function ye(e,t){return!e||!e.roles?!1:t.some(n=>e.roles.includes(n))}function X(){window.dispatchEvent(new CustomEvent("route-change",{detail:{route:E,params:S}}))}function $e(){return S}function I(){return E}function ke(){const e=window.location.pathname,t=j(e);t?(E=t.route,S=t.params):(E=K.find(n=>n.path==="/erc20token")||K[0],S={})}const k=[{address:"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",name:"Alice (Admin)",roles:["admin","holder"],initialBalance:"10000"},{address:"0x70997970C51812dc3A010C7d01b50e0d17dc79C8",name:"Bob (Holder)",roles:["holder"],initialBalance:"1000"},{address:"0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC",name:"Charlie (Holder)",roles:["holder"],initialBalance:"500"},{address:"0x90F79bf6EB2c4f870365E785982E1f101E93b906",name:"Dave (Observer)",roles:[],initialBalance:"0"}],G="balances",xe=!0;let u=null,A=-1;const x={connect:[],disconnect:[],accountChanged:[]};function ie(e,t){x[e]&&x[e].forEach(n=>n(t))}function Ee(e,t){x[e]&&x[e].push(t)}function Se(e,t){x[e]&&(x[e]=x[e].filter(n=>n!==t))}async function L(e=0){if(e<0||e>=k.length)throw new Error(`Invalid account index: ${e}`);const t=k[e];u=t,A=e;try{const n=await fetch("/api/debug/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({user_id:t.address,login:t.name||t.address,roles:t.roles})});if(!n.ok)throw new Error("Failed to authenticate wallet");const a=await n.json();return localStorage.setItem("auth",JSON.stringify(a)),localStorage.setItem("wallet",JSON.stringify({accountIndex:e,address:t.address})),ie("connect",{account:t,index:e,authData:a}),{account:t,index:e,authData:a}}catch(n){throw u=null,A=-1,n}}async function Q(){const e=u;u=null,A=-1,localStorage.removeItem("auth"),localStorage.removeItem("wallet");try{await fetch("/auth/logout",{method:"POST"})}catch{}e&&ie("disconnect",{account:e})}async function Ae(e){return e===A?ce():(await Q(),L(e))}async function le(){const e=localStorage.getItem("wallet");if(e)try{const{accountIndex:t}=JSON.parse(e);if(t>=0&&t<k.length)return L(t)}catch{localStorage.removeItem("wallet")}return null}function Ce(){return u!==null}function ce(){return u}function Te(){return(u==null?void 0:u.address)||null}function Ne(){return A}function Ie(){return k}function Fe(){return(u==null?void 0:u.roles)||[]}function de(e,t){if(!t||!G)return null;const n=t[G];return!n||typeof n!="object"?null:n[e]||0}function ue(e){return u?de(u.address,e):null}function Le(e){if(!u)return`
      <div class="wallet-status disconnected">
        <span class="wallet-dot"></span>
        <span class="wallet-text">Not Connected</span>
        <button onclick="window.showWalletModal()" class="btn btn-primary btn-sm">Connect</button>
      </div>
    `;const t=ue(e),n=t!==null?D(t):"...";return`
    <div class="wallet-status connected">
      <span class="wallet-dot connected"></span>
      <span class="wallet-address" title="${u.address}">
        ${fe(u.address)}
      </span>
      <span class="wallet-balance">${n} ETH</span>
      <button onclick="window.toggleDebugWalletView()" class="btn btn-link btn-sm" title="Show all accounts">Debug</button>
      <button onclick="window.showWalletModal()" class="btn btn-link btn-sm">Switch</button>
      <button onclick="window.disconnectWallet()" class="btn btn-link btn-sm">Disconnect</button>
    </div>
  `}function Be(){return`
    <div id="wallet-modal" class="modal" style="display: none;">
      <div class="modal-backdrop" onclick="window.hideWalletModal()"></div>
      <div class="modal-content">
        <div class="modal-header">
          <h3>Select Account</h3>
          <button onclick="window.hideWalletModal()" class="modal-close">&times;</button>
        </div>
        <div class="modal-body">
          <p>Choose an account to connect:</p>
          <div class="wallet-account-list">
            ${k.map((t,n)=>{const a=n===A;return`
      <div class="wallet-account-row">
        <button
          onclick="window.connectWalletAccount(${n})"
          class="wallet-account-btn ${a?"active":""}"
        >
          <div class="wallet-account-info">
            <span class="wallet-account-name">${t.name||"Account "+n}</span>
            <span class="wallet-account-address">${fe(t.address)}</span>
          </div>
          <div class="wallet-account-roles">
            ${t.roles.map(o=>`<span class="role-badge">${o}</span>`).join("")}
          </div>
          ${a?'<span class="wallet-account-active">Connected</span>':""}
        </button>
        <button
          onclick="window.copyAddress('${t.address}')"
          class="copy-btn"
          title="Copy full address"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
          </svg>
        </button>
      </div>
    `}).join("")}
          </div>
        </div>
      </div>
    </div>
    <style>
      .wallet-status {
        display: flex;
        align-items: center;
        gap: 0.5rem;
      }
      .wallet-dot {
        width: 8px;
        height: 8px;
        border-radius: 50%;
        background: #f59e0b;
      }
      .wallet-dot.connected {
        background: #10b981;
      }
      .wallet-address {
        font-family: monospace;
        font-size: 0.85rem;
        color: rgba(255,255,255,0.9);
      }
      .wallet-balance {
        font-weight: 600;
        color: #10b981;
      }
      .wallet-account-list {
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
      }
      .wallet-account-row {
        display: flex;
        gap: 0.5rem;
        align-items: stretch;
      }
      .wallet-account-btn {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.75rem 1rem;
        border: 1px solid #ddd;
        border-radius: 6px;
        background: #f9f9f9;
        cursor: pointer;
        transition: all 0.2s;
        text-align: left;
        flex: 1;
      }
      .wallet-account-btn:hover {
        background: #e5e5e5;
        border-color: #007bff;
      }
      .wallet-account-btn.active {
        background: #e3f2fd;
        border-color: #007bff;
      }
      .copy-btn {
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 0.5rem;
        border: 1px solid #ddd;
        border-radius: 6px;
        background: #f9f9f9;
        cursor: pointer;
        transition: all 0.2s;
        color: #666;
      }
      .copy-btn:hover {
        background: #e5e5e5;
        border-color: #007bff;
        color: #007bff;
      }
      .copy-btn.copied {
        background: #d4edda;
        border-color: #28a745;
        color: #28a745;
      }
      .wallet-account-info {
        display: flex;
        flex-direction: column;
      }
      .wallet-account-name {
        font-weight: 600;
      }
      .wallet-account-address {
        font-family: monospace;
        font-size: 0.8rem;
        color: #666;
      }
      .wallet-account-roles {
        display: flex;
        gap: 0.25rem;
      }
      .role-badge {
        padding: 0.125rem 0.5rem;
        background: #e5e5e5;
        border-radius: 4px;
        font-size: 0.75rem;
      }
      .wallet-account-active {
        color: #10b981;
        font-size: 0.8rem;
      }
    </style>
  `}function pe(e,t=null){const n=(e==null?void 0:e[G])||{},a=t!==null,o=k.map((i,c)=>{const p=n[i.address]||0,C=c===A;return`
      <tr class="${C?"debug-wallet-connected":""}">
        <td>
          <div class="debug-wallet-account">
            <span class="debug-wallet-name">${i.name||"Account "+c}</span>
            ${C?'<span class="debug-wallet-badge">Connected</span>':""}
          </div>
        </td>
        <td class="debug-wallet-address">
          <code>${i.address}</code>
          <button onclick="window.copyAddress('${i.address}')" class="copy-btn-small" title="Copy">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
              <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
            </svg>
          </button>
        </td>
        <td class="debug-wallet-roles">
          ${i.roles.length>0?i.roles.map(be=>`<span class="role-badge">${be}</span>`).join(" "):'<span class="no-roles">none</span>'}
        </td>
        <td class="debug-wallet-balance">
          <strong>${D(p)}</strong> ETH
        </td>
      </tr>
    `}).join(""),r=k.reduce((i,c)=>i+(n[c.address]||0),0),l=(e==null?void 0:e.total_supply)||0;return`
    <div id="debug-wallet-view" class="debug-wallet-view">
      <div class="debug-wallet-header">
        <h3>Debug Wallet ${a?`<span class="debug-wallet-instance">Instance: ${t.slice(0,8)}...</span>`:'<span class="debug-wallet-no-instance">No instance selected</span>'}</h3>
        <div class="debug-wallet-controls">
          <button onclick="window.toggleDebugWalletFullscreen()" class="debug-wallet-btn" title="Toggle fullscreen">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M8 3H5a2 2 0 0 0-2 2v3m18 0V5a2 2 0 0 0-2-2h-3m0 18h3a2 2 0 0 0 2-2v-3M3 16v3a2 2 0 0 0 2 2h3"></path>
            </svg>
          </button>
          <button onclick="window.hideDebugWalletView()" class="debug-wallet-btn" title="Close">&times;</button>
        </div>
      </div>
      ${a?"":'<div class="debug-wallet-warning">Navigate to an instance to see balances</div>'}
      <div class="debug-wallet-body">
        <table class="debug-wallet-table">
          <thead>
            <tr>
              <th>Account</th>
              <th>Address</th>
              <th>Roles</th>
              <th>Balance</th>
            </tr>
          </thead>
          <tbody>
            ${o}
          </tbody>
          <tfoot>
            <tr>
              <td colspan="3"><strong>Total Balances</strong></td>
              <td class="debug-wallet-balance"><strong>${D(r)}</strong> ETH</td>
            </tr>
            <tr>
              <td colspan="3"><strong>Total Supply</strong></td>
              <td class="debug-wallet-balance"><strong>${D(l)}</strong> ETH</td>
            </tr>
          </tfoot>
        </table>
      </div>
    </div>
    <style>
      .debug-wallet-view {
        position: fixed;
        bottom: 1rem;
        right: 1rem;
        background: white;
        border-radius: 8px;
        box-shadow: 0 4px 20px rgba(0,0,0,0.2);
        z-index: 1000;
        min-width: 400px;
        min-height: 200px;
        width: 650px;
        max-width: 90vw;
        max-height: 80vh;
        overflow: hidden;
        display: flex;
        flex-direction: column;
        resize: both;
        overflow: auto;
      }
      .debug-wallet-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.75rem 1rem;
        background: #24292e;
        color: white;
        cursor: move;
        user-select: none;
      }
      .debug-wallet-header h3 {
        margin: 0;
        font-size: 1rem;
        display: flex;
        align-items: center;
        gap: 0.5rem;
        background: rgba(255,255,255,0.15);
        padding: 0.25rem 0.75rem;
        border-radius: 6px;
      }
      .debug-wallet-instance {
        font-size: 0.8rem;
        font-weight: normal;
        color: #10b981;
        font-family: monospace;
      }
      .debug-wallet-no-instance {
        font-size: 0.8rem;
        font-weight: normal;
        color: #f59e0b;
      }
      .debug-wallet-warning {
        padding: 0.5rem 1rem;
        background: #fef3cd;
        color: #856404;
        font-size: 0.85rem;
      }
      .debug-wallet-controls {
        display: flex;
        gap: 0.25rem;
      }
      .debug-wallet-btn {
        background: transparent;
        border: none;
        color: rgba(255,255,255,0.7);
        cursor: pointer;
        padding: 0.25rem 0.5rem;
        font-size: 1.25rem;
        line-height: 1;
        border-radius: 4px;
        display: flex;
        align-items: center;
        justify-content: center;
      }
      .debug-wallet-btn:hover {
        background: rgba(255,255,255,0.1);
        color: white;
      }
      .debug-wallet-view.fullscreen {
        top: 1rem !important;
        left: 1rem !important;
        right: 1rem !important;
        bottom: 1rem !important;
        width: auto !important;
        height: auto !important;
        max-width: none;
        max-height: none;
      }
      .debug-wallet-body {
        padding: 1rem;
        overflow: auto;
      }
      .debug-wallet-table {
        width: 100%;
        border-collapse: collapse;
        font-size: 0.9rem;
        table-layout: fixed;
      }
      .debug-wallet-table th {
        text-align: left;
        padding: 0.5rem;
        border-bottom: 2px solid #ddd;
        font-weight: 600;
        color: #555;
      }
      .debug-wallet-table th:nth-child(1) { width: 20%; }
      .debug-wallet-table th:nth-child(2) { width: 40%; }
      .debug-wallet-table th:nth-child(3) { width: 18%; }
      .debug-wallet-table th:nth-child(4) { width: 22%; }
      .debug-wallet-address {
        overflow: hidden;
      }
      .debug-wallet-address code {
        display: inline-block;
        max-width: calc(100% - 30px);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        vertical-align: middle;
        background: #f5f5f5;
        padding: 0.25rem 0.5rem;
        border-radius: 4px;
      }
      .debug-wallet-table td {
        padding: 0.5rem;
        border-bottom: 1px solid #eee;
        vertical-align: middle;
      }
      .debug-wallet-table tfoot td {
        border-top: 2px solid #ddd;
        border-bottom: none;
        background: #f9f9f9;
      }
      .debug-wallet-connected {
        background: #e8f5e9;
      }
      .debug-wallet-account {
        display: flex;
        align-items: center;
        gap: 0.5rem;
      }
      .debug-wallet-name {
        font-weight: 500;
      }
      .debug-wallet-badge {
        font-size: 0.7rem;
        padding: 0.125rem 0.375rem;
        background: #10b981;
        color: white;
        border-radius: 4px;
      }
      .debug-wallet-address {
        font-family: monospace;
        font-size: 0.8rem;
        display: flex;
        align-items: center;
        gap: 0.5rem;
      }
      .copy-btn-small {
        padding: 0.25rem;
        border: none;
        background: transparent;
        cursor: pointer;
        color: #666;
        display: flex;
        align-items: center;
      }
      .copy-btn-small:hover {
        color: #007bff;
      }
      .debug-wallet-roles {
        display: flex;
        gap: 0.25rem;
        flex-wrap: wrap;
      }
      .no-roles {
        color: #999;
        font-style: italic;
      }
      .debug-wallet-balance {
        text-align: right;
        font-family: monospace;
      }
    </style>
  `}function fe(e){return e?e.length<=12?e:e.slice(0,6)+"..."+e.slice(-4):""}function D(e){if(e==null)return"0";const t=typeof e=="string"?parseFloat(e):e;return t===0?"0":t<1e-4?"<0.0001":t<1?t.toFixed(4):t<1e3?t.toFixed(2):t.toLocaleString(void 0,{maximumFractionDigits:2})}window.showWalletModal=function(){const e=document.getElementById("wallet-modal");e&&(e.style.display="flex")};window.hideWalletModal=function(){const e=document.getElementById("wallet-modal");e&&(e.style.display="none")};window.connectWalletAccount=async function(e){try{await L(e),window.hideWalletModal(),window.dispatchEvent(new CustomEvent("auth-change"))}catch(t){console.error("Wallet connection failed:",t),alert("Failed to connect wallet: "+t.message)}};window.disconnectWallet=async function(){await Q(),window.dispatchEvent(new CustomEvent("auth-change"))};window.showDebugWalletView=function(){var o;const e=window.currentInstanceState||{},t=((o=window.currentInstance)==null?void 0:o.id)||null,n=document.getElementById("debug-wallet-container");n&&n.remove();const a=document.createElement("div");a.id="debug-wallet-container",a.innerHTML=pe(e,t),document.body.appendChild(a),De()};function De(){const e=document.getElementById("debug-wallet-view"),t=e==null?void 0:e.querySelector(".debug-wallet-header");if(!e||!t)return;let n=!1,a,o,r,l;t.addEventListener("mousedown",i=>{if(i.target.closest("button"))return;n=!0,a=i.clientX,o=i.clientY;const c=e.getBoundingClientRect();r=c.left,l=c.top,e.style.bottom="auto",e.style.right="auto",e.style.left=r+"px",e.style.top=l+"px",i.preventDefault()}),document.addEventListener("mousemove",i=>{if(!n)return;const c=i.clientX-a,p=i.clientY-o;e.style.left=r+c+"px",e.style.top=l+p+"px"}),document.addEventListener("mouseup",()=>{n=!1})}window.hideDebugWalletView=function(){const e=document.getElementById("debug-wallet-container");e&&e.remove()};window.toggleDebugWalletFullscreen=function(){const e=document.getElementById("debug-wallet-view");e&&e.classList.toggle("fullscreen")};window.toggleDebugWalletView=function(){document.getElementById("debug-wallet-container")?window.hideDebugWalletView():window.showDebugWalletView()};window.copyAddress=async function(e){try{await navigator.clipboard.writeText(e),document.querySelectorAll(".copy-btn").forEach(n=>{var a;(a=n.onclick)!=null&&a.toString().includes(e)&&(n.classList.add("copied"),setTimeout(()=>n.classList.remove("copied"),1500))})}catch{const n=document.createElement("textarea");n.value=e,document.body.appendChild(n),n.select(),document.execCommand("copy"),document.body.removeChild(n)}};typeof window<"u"&&window.addEventListener("DOMContentLoaded",async()=>{if(!await le()&&xe&&k.length>0)try{await L(0),window.dispatchEvent(new CustomEvent("auth-change"))}catch(t){console.error("Auto-connect failed:",t)}});const g={connect:L,disconnect:Q,switchAccount:Ae,restore:le,isConnected:Ce,getAccount:ce,getAddress:Te,getAccountIndex:Ne,getAccounts:Ie,getAccountRoles:Fe,getBalance:de,getMyBalance:ue,createWalletStatus:Le,createWalletModal:Be,createDebugWalletView:pe,on:Ee,off:Se},W={brand:"erc20-token",items:[{label:"erc20-token",path:"/erc20token",icon:""},{label:"New",path:"/erc20token/new",icon:"+"},{label:"Admin",path:"/admin",icon:""}]};let f=null,H=!1;async function me(){if(!H){H=!0;try{const e={},t=he();t&&(e.Authorization=`Bearer ${t}`);const n=await fetch("/api/navigation",{headers:e});n.ok?f=await n.json():f=W}catch{f=W}finally{H=!1}}}async function ge(){f||await me();const e=window.location.pathname;Oe();const t=(f==null?void 0:f.items)||W.items;return`
    <nav class="navigation">
      <div class="nav-brand">
        <a href="/erc20token" onclick="handleNavClick(event, '/erc20token')">
          ${(f==null?void 0:f.brand)||W.brand}
        </a>
      </div>
      <ul class="nav-menu">
        ${t.map(o=>`
            <li class="${e===o.path||o.path!=="/"&&e.startsWith(o.path)?"active":""}">
              <a href="${o.path}" onclick="handleNavClick(event, '${o.path}')">
                ${o.icon?`<span class="icon">${o.icon}</span>`:""}
                ${o.label}
              </a>
            </li>
          `).join("")}
      </ul>
      <div class="nav-user">
        ${g.createWalletStatus(window.currentInstanceState)}
      </div>
    </nav>
    ${g.createWalletModal()}
  `}window.showLoginModal=function(){const e=document.getElementById("login-modal");e&&(e.style.display="flex")};window.hideLoginModal=function(){const e=document.getElementById("login-modal");e&&(e.style.display="none")};window.handleRoleLogin=async function(e){try{const t=await fetch("/api/debug/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({login:"pilot-user",roles:e})});if(!t.ok)throw new Error("Login failed");const n=await t.json();localStorage.setItem("auth",JSON.stringify(n)),hideLoginModal(),f=null,window.dispatchEvent(new CustomEvent("auth-change")),await P()}catch(t){console.error("Login error:",t),alert("Login failed. Please try again.")}};window.handleNavClick=function(e,t){e.preventDefault(),m(t)};window.handleLogout=async function(){try{const e=he();e&&await fetch("/auth/logout",{method:"POST",headers:{Authorization:`Bearer ${e}`}})}catch(e){console.error("Logout error:",e)}localStorage.removeItem("auth"),f=null,window.dispatchEvent(new CustomEvent("auth-change")),await P(),m("/erc20token")};function Oe(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).user}catch{return null}return null}function he(){const e=localStorage.getItem("auth");if(e)try{return JSON.parse(e).token}catch{return null}return null}async function P(){f=null,await me();const e=document.getElementById("nav");e&&(e.innerHTML=await ge())}window.addEventListener("auth-change",async()=>{await P()});window.addEventListener("route-change",()=>{const e=window.location.pathname;document.querySelectorAll(".nav-menu li").forEach(t=>{t.classList.remove("active")}),document.querySelectorAll(".nav-menu a").forEach(t=>{const n=t.getAttribute("href");(n===e||n!=="/"&&e.startsWith(n))&&t.parentElement.classList.add("active")})});let re=[];async function Me(){try{const e=await fetch("/api/views");return e.ok?(re=await e.json(),re):(console.warn("Failed to load view definitions, using defaults"),[])}catch(e){return console.error("Error loading views:",e),[]}}window.wallet=g;const b="",O=18,R="ETH";function _(e){if(e==null)return"";const t=String(e);if(t==="0")return"0";const n=t.padStart(O+1,"0"),a=n.slice(0,-O)||"0",o=n.slice(-O).replace(/0+$/,"");return o?`${a}.${o}`:a}function je(e){if(e==null||e==="")return"0";const t=typeof e=="string"?parseFloat(e):e;if(isNaN(t))return"0";const n=BigInt(10**O),a=BigInt(Math.floor(t)),o=t-Math.floor(t),r=BigInt(Math.round(o*Number(n)));return String(a*n+r)}function Z(e){return["amount","value","balance","total_supply","allowance"].some(n=>e.toLowerCase().includes(n))}function We(e){const t={...e};for(const[n,a]of Object.entries(t))Z(n)&&(typeof a=="number"||!isNaN(parseFloat(a)))&&(t[n]=je(a));return t}function z(e){return!!(typeof e=="number"||typeof e=="string"&&/^\d+$/.test(e))}function ee(e){const t={...e};for(const[n,a]of Object.entries(t))if(Z(n)){if(z(a))t[n]=_(a);else if(typeof a=="object"&&a!==null){t[n]={};for(const[o,r]of Object.entries(a))if(z(r))t[n][o]=_(r);else if(typeof r=="object"&&r!==null){t[n][o]={};for(const[l,i]of Object.entries(r))t[n][o][l]=z(i)?_(i):i}else t[n][o]=r}}return t}let d=null,w=null,$=[],s=null;const V=[{id:"transfer",name:"Transfer",description:"Transfer tokens from sender to recipient",fields:[{name:"from",label:"From Address",type:"address",required:!0,autoFill:"wallet",placeholder:"Sender address",defaultValue:""},{name:"to",label:"To Address",type:"address",required:!0,autoFill:"",placeholder:"Recipient address",defaultValue:""},{name:"amount",label:"Amount",type:"amount",required:!0,autoFill:"",placeholder:"Amount to transfer",defaultValue:""}]},{id:"approve",name:"Approve",description:"Approve spender to transfer tokens on owner's behalf",fields:[{name:"owner",label:"Owner Address",type:"address",required:!0,autoFill:"wallet",placeholder:"Token owner",defaultValue:""},{name:"spender",label:"Spender Address",type:"address",required:!0,autoFill:"",placeholder:"Address to approve",defaultValue:""},{name:"amount",label:"Allowance Amount",type:"amount",required:!0,autoFill:"",placeholder:"Amount to approve",defaultValue:""}]},{id:"transfer_from",name:"Transfer From",description:"Transfer tokens using allowance (delegated transfer)",fields:[{name:"from",label:"From Address",type:"address",required:!0,autoFill:"",placeholder:"Owner address",defaultValue:""},{name:"to",label:"To Address",type:"address",required:!0,autoFill:"",placeholder:"Recipient address",defaultValue:""},{name:"caller",label:"Caller Address",type:"address",required:!0,autoFill:"wallet",placeholder:"Your address (must have allowance)",defaultValue:""},{name:"amount",label:"Amount",type:"amount",required:!0,autoFill:"",placeholder:"Amount to transfer",defaultValue:""}]},{id:"mint",name:"Mint",description:"Create new tokens and add to recipient balance",fields:[{name:"to",label:"Recipient Address",type:"address",required:!0,autoFill:"",placeholder:"Address to mint to",defaultValue:""},{name:"amount",label:"Amount",type:"amount",required:!0,autoFill:"",placeholder:"Amount to mint",defaultValue:""}]},{id:"burn",name:"Burn",description:"Destroy tokens from holder's balance",fields:[{name:"from",label:"From Address",type:"address",required:!0,autoFill:"wallet",placeholder:"Address to burn from",defaultValue:""},{name:"amount",label:"Amount",type:"amount",required:!0,autoFill:"",placeholder:"Amount to burn",defaultValue:""}]}];function Re(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);if(t.expires_at&&new Date(t.expires_at)>new Date)return w=t.token,d=t.user,!0;localStorage.removeItem("auth")}catch{localStorage.removeItem("auth")}return!1}function te(e){localStorage.setItem("auth",JSON.stringify(e)),w=e.token,d=e.user,window.dispatchEvent(new CustomEvent("auth-change"))}function B(){localStorage.removeItem("auth"),w=null,d=null,window.dispatchEvent(new CustomEvent("auth-change"))}function qe(){const e=localStorage.getItem("auth");if(e)try{const t=JSON.parse(e);return w=t.token,d=t.user,!0}catch{return!1}return w=null,d=null,!1}window.addEventListener("auth-change",()=>{qe()});function v(){const e={"Content-Type":"application/json"};return w&&(e.Authorization=`Bearer ${w}`),e}async function T(e){if(e.status===401)throw B(),N("Session expired. Please log in again."),new Error("Unauthorized");if(!e.ok){const t=await e.json().catch(()=>({}));throw new Error(t.message||e.statusText)}return e.json()}const h={async getMe(){const e=await fetch(`${b}/auth/me`,{headers:v()});return T(e)},async logout(){await fetch(`${b}/auth/logout`,{method:"POST",headers:v()}),B()},async listInstances(){const e=await fetch(`${b}/admin/instances`,{headers:v()});return T(e)},async getInstance(e){const t=await fetch(`${b}/api/erc20token/${e}`,{headers:v()});return T(t)},async createInstance(e={}){const t=await fetch(`${b}/api/erc20token`,{method:"POST",headers:v(),body:JSON.stringify(e)});return T(t)},async executeTransition(e,t,n={}){const a=We(n),o=await fetch(`${b}/api/${e}`,{method:"POST",headers:v(),body:JSON.stringify({aggregate_id:t,data:a})});return T(o)}};window.api=h;Object.defineProperty(window,"currentInstance",{get:function(){return s}});window.setAuthToken=function(e){w=e};window.saveAuth=te;window.clearAuth=B;function N(e){const t=document.getElementById("app"),n=t.querySelector(".alert-error");n&&n.remove();const a=document.createElement("div");a.className="alert alert-error",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),5e3)}function ne(e){const t=document.getElementById("app"),n=t.querySelector(".alert-success");n&&n.remove();const a=document.createElement("div");a.className="alert alert-success",a.textContent=e,t.insertBefore(a,t.firstChild),setTimeout(()=>a.remove(),3e3)}function ae(e){if(!e)return"unknown";for(const[t,n]of Object.entries(e))if(n>0)return t;return"unknown"}function oe(e){return`<span class="badge ${`badge-${e.toLowerCase().replace(/_/g,"-")}`}">${e.replace(/_/g," ")}</span>`}async function J(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <h1>erc20-token</h1>
        <button class="btn btn-primary" onclick="handleCreateNew()">+ New</button>
      </div>
      <div id="instances-list" class="entity-list">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{$=(await h.listInstances()).instances||[],Pe()}catch{document.getElementById("instances-list").innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `}}function Pe(){const e=document.getElementById("instances-list");if(e){if($.length===0){e.innerHTML=`
      <div class="empty-state">
        <h3>No instances yet</h3>
        <p>Create your first instance to get started.</p>
        <button class="btn btn-primary" onclick="handleCreateNew()" style="margin-top: 1rem">+ Create New</button>
      </div>
    `;return}e.innerHTML=$.map(t=>{const n=ae(t.state||t.places);return`
      <div class="entity-card" onclick="navigate('/erc20token/${t.id}')">
        <div class="entity-info">
          <h3>${t.id}</h3>
          <div class="entity-meta">
            ${oe(n)} &middot; Version ${t.version||0}
          </div>
        </div>
        <div class="entity-actions">
          <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); navigate('/erc20token/${t.id}')">
            View
          </button>
        </div>
      </div>
    `}).join("")}}async function Ve(){const t=$e().id,n=document.getElementById("app");n.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/erc20token')" style="margin-left: -0.5rem">
            &larr; Back to List
          </button>
          <h1 style="margin-top: 0.5rem">Instance: ${t}</h1>
        </div>
      </div>
      <div id="instance-detail">
        <div class="loading">Loading...</div>
      </div>
    </div>
  `;try{const a=await h.getInstance(t);s={id:a.aggregate_id||t,version:a.version,state:a.state,displayState:ee(a.state),places:a.places,enabled:a.enabled||a.enabled_transitions||[]},window.currentInstanceState=s.state,F()}catch(a){document.getElementById("instance-detail").innerHTML=`
      <div class="alert alert-error">Failed to load instance: ${a.message}</div>
    `}}function F(){const e=document.getElementById("instance-detail");if(!e||!s)return;const t=ae(s.places),n=s.enabled||[],a=V;e.innerHTML=`
    <div class="card">
      <div class="card-header">Status</div>
      <div class="detail-list">
        <div class="detail-field">
          <dt>ID</dt>
          <dd><code>${s.id}</code></dd>
        </div>
        <div class="detail-field">
          <dt>Status</dt>
          <dd>${oe(t)}</dd>
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
      <div class="card-header">Current State${` (${R})`}</div>
      <div class="detail-list">
        ${He(s.displayState||s.state)}
      </div>
    </div>
  `}function He(e){return!e||Object.keys(e).length===0?'<p style="color: #999;">No state data</p>':Object.entries(e).map(([t,n])=>{if(typeof n=="object"&&n!==null){const a=Object.entries(n);return a.length===0?`
          <div class="detail-field">
            <dt>${U(t)}</dt>
            <dd><span style="color: #999;">Empty</span></dd>
          </div>
        `:`
        <div class="detail-field">
          <dt>${U(t)}</dt>
          <dd>
            <div class="nested-state">
              ${a.map(([o,r])=>{if(typeof r=="object"&&r!==null){const l=Object.entries(r);return l.length===0?`
                      <div class="state-entry">
                        <span class="state-key">${o}</span>
                        <span class="state-value" style="color: #999;">Empty</span>
                      </div>
                    `:`
                    <div class="state-entry nested-group">
                      <span class="state-key">${o}</span>
                      <div class="nested-state" style="margin-left: 1rem;">
                        ${l.map(([i,c])=>`
                          <div class="state-entry">
                            <span class="state-key">${i}</span>
                            <span class="state-value">${Y(t,c)}</span>
                          </div>
                        `).join("")}
                      </div>
                    </div>
                  `}return`
                  <div class="state-entry">
                    <span class="state-key">${o}</span>
                    <span class="state-value">${Y(t,r)}</span>
                  </div>
                `}).join("")}
            </div>
          </dd>
        </div>
      `}return`
      <div class="detail-field">
        <dt>${U(t)}</dt>
        <dd>${Y(t,n)}</dd>
      </div>
    `}).join("")}function U(e){return e.replace(/_/g," ").replace(/\b\w/g,t=>t.toUpperCase())}function Y(e,t){return Z(e)&&R?`<strong>${t}</strong> ${R}`:`<strong>${t}</strong>`}function _e(){if(typeof g<"u"&&g.getAccount){const e=g.getAccount();return(e==null?void 0:e.address)||null}return null}function ze(e,t){return e?e==="wallet"?_e()||"":e==="user"?(d==null?void 0:d.id)||(d==null?void 0:d.login)||"":(e.startsWith("balances.")||e.includes("."),""):""}function Je(){return`
    <div id="action-modal" class="modal" style="display: none;">
      <div class="modal-backdrop" onclick="hideActionModal()"></div>
      <div class="modal-content">
        <div class="modal-header">
          <h3 id="action-modal-title">Execute Action</h3>
          <button onclick="hideActionModal()" class="modal-close">&times;</button>
        </div>
        <div class="modal-body">
          <form id="action-form" onsubmit="handleActionSubmit(event)">
            <div id="action-form-fields"></div>
            <div class="form-actions">
              <button type="submit" class="btn btn-primary">Execute</button>
              <button type="button" class="btn btn-secondary" onclick="hideActionModal()">Cancel</button>
            </div>
          </form>
        </div>
      </div>
    </div>
    <style>
      .modal {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        z-index: 1000;
        display: flex;
        align-items: center;
        justify-content: center;
      }
      .modal-backdrop {
        position: absolute;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background: rgba(0, 0, 0, 0.5);
      }
      .modal-content {
        position: relative;
        background: white;
        border-radius: 8px;
        padding: 0;
        min-width: 400px;
        max-width: 90%;
        box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
      }
      .modal-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 1rem 1.5rem;
        border-bottom: 1px solid #eee;
      }
      .modal-header h3 {
        margin: 0;
        font-size: 1.25rem;
      }
      .modal-close {
        background: none;
        border: none;
        font-size: 1.5rem;
        cursor: pointer;
        color: #666;
        padding: 0;
        line-height: 1;
      }
      .modal-close:hover {
        color: #333;
      }
      .modal-body {
        padding: 1.5rem;
      }
      .address-input-wrapper {
        position: relative;
      }
      .address-picker-btn {
        position: absolute;
        right: 4px;
        top: 50%;
        transform: translateY(-50%);
        background: #f0f0f0;
        border: 1px solid #ddd;
        border-radius: 4px;
        padding: 4px 8px;
        font-size: 0.85rem;
        cursor: pointer;
      }
      .address-picker-btn:hover {
        background: #e0e0e0;
      }
      .field-description {
        font-size: 0.85rem;
        color: #666;
        margin-top: 0.25rem;
      }
    </style>
  `}let q=null;function Ue(e){const t=V.find(a=>a.id===e);if(!t)return;q=e,document.getElementById("action-modal-title").textContent=t.name;const n=t.fields.map(a=>{const r=ze(a.autoFill,s==null?void 0:s.state)||a.defaultValue||"",l=a.required?"required":"";let i="";if(a.type==="amount")i=`
        <input
          type="number"
          name="${a.name}"
          value="${r}"
          placeholder="${a.placeholder||"Amount"}"
          step="any"
          ${l}
          class="form-control"
        />
        ${`<span class="field-description">Amount in ${R}</span>`}
      `;else if(a.type==="address"){const c=Ye();c.length>0?i=`
          <select name="${a.name}" ${l} class="form-control">
            <option value="">Select address...</option>
            ${c.map(p=>`
              <option value="${p.address}" ${p.address===r?"selected":""}>
                ${p.name||"Account"} (${p.address.slice(0,8)}...${p.address.slice(-6)})
              </option>
            `).join("")}
          </select>
        `:i=`
          <input
            type="text"
            name="${a.name}"
            value="${r}"
            placeholder="${a.placeholder||"0x..."}"
            ${l}
            class="form-control"
          />
        `}else a.type==="hidden"?i=`<input type="hidden" name="${a.name}" value="${r}" />`:i=`
        <input
          type="${a.type==="number"?"number":"text"}"
          name="${a.name}"
          value="${r}"
          placeholder="${a.placeholder||""}"
          ${l}
          class="form-control"
        />
      `;return a.type==="hidden"?i:`
      <div class="form-field">
        <label>${a.label}${a.required?" *":""}</label>
        ${i}
      </div>
    `}).join("");document.getElementById("action-form-fields").innerHTML=n,document.getElementById("action-modal").style.display="flex"}window.hideActionModal=function(){document.getElementById("action-modal").style.display="none",q=null};window.handleActionSubmit=async function(e){var l;if(e.preventDefault(),!q||!s)return;const t=q,n=s.id,a=e.target,o=new FormData(a),r={};for(const[i,c]of o.entries()){const p=(l=V.find(C=>C.id===t))==null?void 0:l.fields.find(C=>C.name===i);p&&(p.type==="amount"||p.type==="number")?r[i]=parseFloat(c)||0:r[i]=c}hideActionModal();try{const i=await h.executeTransition(t,n,r);s={...s,version:i.version,state:i.state,displayState:ee(i.state),places:i.state,enabled:i.enabled||[]},window.currentInstanceState=s.state,F(),ne(`Action "${t}" completed!`)}catch(i){N(`Failed to execute ${t}: ${i.message}`)}};function Ye(){return typeof g<"u"&&g.getAccounts?g.getAccounts()||[]:[]}window.showAddressPicker=function(e){if(typeof g>"u"||!g.getAccounts)return;const t=g.getAccounts();if(!t||t.length===0)return;const n=document.querySelector(".address-picker-dropdown");n&&n.remove();const a=document.querySelector(`[name="${e}"]`);if(!a)return;const o=a.getBoundingClientRect(),r=document.createElement("div");r.className="address-picker-dropdown",r.style.cssText=`
    position: fixed;
    top: ${o.bottom+4}px;
    left: ${o.left}px;
    width: ${o.width}px;
    background: white;
    border: 1px solid #ddd;
    border-radius: 4px;
    box-shadow: 0 4px 12px rgba(0,0,0,0.15);
    z-index: 2000;
    max-height: 200px;
    overflow-y: auto;
  `,r.innerHTML=t.map(l=>`
    <div class="address-picker-option" onclick="selectAddress('${e}', '${l.address}')" style="
      padding: 8px 12px;
      cursor: pointer;
      border-bottom: 1px solid #eee;
    ">
      <div style="font-weight: 500;">${l.name||"Account"}</div>
      <div style="font-size: 0.85rem; color: #666; font-family: monospace;">${l.address.slice(0,10)}...${l.address.slice(-8)}</div>
    </div>
  `).join(""),document.body.appendChild(r),setTimeout(()=>{document.addEventListener("click",function l(i){r.contains(i.target)||(r.remove(),document.removeEventListener("click",l))})},0)};window.selectAddress=function(e,t){const n=document.querySelector(`[name="${e}"]`);n&&(n.value=t,n.dispatchEvent(new Event("input",{bubbles:!0})));const a=document.querySelector(".address-picker-dropdown");a&&a.remove()};async function Ke(){const e=document.getElementById("app");e.innerHTML=`
    <div class="page">
      <div class="page-header">
        <div>
          <button class="btn btn-link" onclick="navigate('/erc20token')" style="margin-left: -0.5rem">
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
            <button type="button" class="btn btn-secondary" onclick="navigate('/erc20token')">Cancel</button>
          </div>
        </form>
      </div>
    </div>
  `}async function Xe(){const e=document.getElementById("app");e.innerHTML=`
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
  `;try{const[t,n]=await Promise.all([fetch(`${b}/admin/stats`,{headers:v()}).then(o=>o.json()).catch(()=>null),h.listInstances()]);t?document.getElementById("admin-stats").innerHTML=`
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
      `:document.getElementById("admin-stats").innerHTML="",$=n.instances||[];const a=document.getElementById("admin-instances").querySelector(".loading");a&&(a.outerHTML=$.length>0?`<table class="table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Status</th>
                <th>Version</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              ${$.slice(0,20).map(o=>{const r=ae(o.state||o.places);return`
                  <tr>
                    <td><code>${o.id}</code></td>
                    <td>${oe(r)}</td>
                    <td>${o.version||0}</td>
                    <td><button class="btn btn-sm btn-link" onclick="navigate('/erc20token/${o.id}')">View</button></td>
                  </tr>
                `}).join("")}
            </tbody>
          </table>`:'<p style="color: #666; padding: 1rem;">No instances yet.</p>')}catch(t){N("Failed to load admin data: "+t.message)}}window.navigate=m;window.handleCreateNew=async function(){m("/erc20token/new")};window.handleSubmitCreate=async function(e){e.preventDefault();try{const t=await h.createInstance({});ne("Instance created successfully!"),m(`/erc20token/${t.aggregate_id||t.id}`)}catch(t){N("Failed to create: "+t.message)}};window.handleTransition=async function(e){if(!s)return;const t=V.find(n=>n.id===e);if(t&&t.fields&&t.fields.length>0){Ue(e);return}try{const n=await h.executeTransition(e,s.id);s={...s,version:n.version,state:n.state,displayState:ee(n.state),places:n.state,enabled:n.enabled||[]},window.currentInstanceState=s.state,F(),ne(`Action "${e}" completed!`)}catch(n){N(`Failed to execute ${e}: ${n.message}`)}};function se(e){var a;const t=((a=e.detail)==null?void 0:a.route)||I();if(!t){J();return}const n=t.path;n==="/erc20token"||n==="/"?J():n==="/erc20token/new"?Ke():n==="/erc20token/:id"?Ve():n==="/admin"||n.startsWith("/admin")?Xe():J()}async function Ge(){const e=new URLSearchParams(window.location.search),t=e.get("token"),n=e.get("expires_at");if(t){w=t;try{const a=await h.getMe();te({token:t,expires_at:n,user:a}),window.history.replaceState({},"",window.location.pathname),await P()}catch{B(),N("Failed to complete login")}}}async function Qe(){Re(),await Ge(),await Me();const e=document.getElementById("nav");e.innerHTML=await ge();const t=document.createElement("div");t.innerHTML=Je(),document.body.appendChild(t),window.addEventListener("route-change",se),ke(),se({detail:{route:I()}})}let y=null,M=null;function we(){const t=`${window.location.protocol==="https:"?"wss:":"ws:"}//${window.location.host}/ws`;y=new WebSocket(t),y.onopen=()=>{console.log("[Debug] WebSocket connected")},y.onmessage=n=>{try{const a=JSON.parse(n.data);a.id==="session"&&a.type==="session"?(M=(typeof a.data=="string"?JSON.parse(a.data):a.data).session_id,console.log("[Debug] Session ID:",M)):a.type==="eval"&&Ze(a)}catch(a){console.error("[Debug] Failed to parse message:",a)}},y.onclose=()=>{console.log("[Debug] WebSocket disconnected, reconnecting in 3s..."),M=null,setTimeout(we,3e3)},y.onerror=n=>{console.error("[Debug] WebSocket error:",n)}}async function Ze(e){try{const n=(typeof e.data=="string"?JSON.parse(e.data):e.data).code,o=await new Function("return (async () => { "+n+" })()")(),r={type:"response",id:e.id,data:{result:o,type:typeof o}};y.send(JSON.stringify(r))}catch(t){const n={type:"response",id:e.id,data:{error:t.message}};y.send(JSON.stringify(n))}}window.debugSessionId=()=>M;window.debugWs=()=>y;window.pilot={async list(){return m("/erc20token"),await this.waitFor(".entity-card, .empty-state",5e3).catch(()=>{}),$},newForm(){return m("/erc20token/new"),this.waitForRender()},async view(e){return m(`/erc20token/${e}`),await this.waitForRender(),s},admin(){return m("/admin"),this.waitForRender()},async create(e={}){const t=await h.createInstance(e),n=t.aggregate_id||t.id;return m(`/erc20token/${n}`),await this.waitForRender(),{id:n,...t}},getCurrentInstance(){return s},getInstances(){return $},async refresh(){if(!s)throw new Error("No current instance");const e=await h.getInstance(s.id);return s={id:e.aggregate_id||s.id,version:e.version,state:e.state,places:e.places,enabled:e.enabled||e.enabled_transitions||[]},F(),s},async action(e,t={}){if(!s)throw new Error("No current instance - navigate to detail page first");const n=await h.executeTransition(e,s.id,t);return s={...s,version:n.version,state:n.state,places:n.state,enabled:n.enabled||[]},F(),{success:!0,state:s.places,enabled:s.enabled}},isEnabled(e){return s?(s.enabled||[]).includes(e):!1},getEnabled(){return(s==null?void 0:s.enabled)||[]},fill(e,t){const n=document.querySelector(`[name="${e}"]`);if(!n)throw new Error(`No input found with name: ${e}`);return n.value=t,n.dispatchEvent(new Event("input",{bubbles:!0})),this},async submit(){const e=document.querySelector("form");if(!e)throw new Error("No form found on page");const t=new Event("submit",{bubbles:!0,cancelable:!0});return e.dispatchEvent(t),await this.waitForRender(),s},getText(e){const t=document.querySelector(e);return t?t.textContent.trim():null},exists(e){return document.querySelector(e)!==null},getButtons(){return Array.from(document.querySelectorAll("button")).map(e=>({text:e.textContent.trim(),disabled:e.disabled,className:e.className}))},async clickButton(e){const t=document.querySelectorAll("button");for(const n of t)if(n.textContent.trim()===e&&!n.disabled)return n.click(),await this.waitForRender(),!0;throw new Error(`No enabled button found with text: ${e}`)},getState(){return(s==null?void 0:s.places)||null},getStatus(){if(!(s!=null&&s.places))return null;for(const[e,t]of Object.entries(s.places))if(t>0)return e;return null},getRoute(){return I()},getUser(){return d},isAuthenticated(){return w!==null},waitForRender(e=50){return new Promise(t=>setTimeout(t,e))},async waitFor(e,t=5e3){const n=Date.now();for(;Date.now()-n<t;){if(document.querySelector(e))return document.querySelector(e);await this.waitForRender(50)}throw new Error(`Timeout waiting for: ${e}`)},async waitForState(e,t=5e3){var a;const n=Date.now();for(;Date.now()-n<t;){if(((a=s==null?void 0:s.places)==null?void 0:a[e])>0)return s;await this.waitForRender(100)}throw new Error(`Timeout waiting for state: ${e}`)},debug(){return console.log("=== Pilot Debug ==="),console.log("Route:",I()),console.log("User:",d),console.log("Instance:",s),console.log("Enabled:",s==null?void 0:s.enabled),console.log("State:",s==null?void 0:s.places),{route:I(),user:d,instance:s}},async getEvents(){if(!s)throw new Error("No current instance");const e=await fetch(`${b}/api/erc20token/${s.id}/events`,{headers:v()});return(await T(e)).events||[]},async getEventCount(){return(await this.getEvents()).length},async getLastEvent(){const e=await this.getEvents();return e.length>0?e[e.length-1]:null},async replayTo(e){if(!s)throw new Error("No current instance");const n=(await this.getEvents()).filter(o=>(o.version||o.sequence)<=e),a={};for(const o of n)o.state&&Object.assign(a,o.state);return{version:e,events:n,places:a}},async loginAs(e){const t=typeof e=="string"?[e]:e,a=await(await fetch(`${b}/api/debug/login`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({login:"pilot-user",roles:t})})).json();return te(a),await this.waitForRender(100),a},logout(){return B(),this.waitForRender()},getRoles(){return(d==null?void 0:d.roles)||[]},hasRole(e){return this.getRoles().includes(e)},assertState(e){const t=this.getStatus();if(t!==e)throw new Error(`Expected state '${e}', got '${t}'`);return this},assertEnabled(e){if(!this.isEnabled(e)){const t=this.getEnabled();throw new Error(`Expected '${e}' to be enabled. Enabled: [${t.join(", ")}]`)}return this},assertDisabled(e){if(this.isEnabled(e))throw new Error(`Expected '${e}' to be disabled, but it is enabled`);return this},assertExists(e){if(!this.exists(e))throw new Error(`Expected element '${e}' to exist`);return this},assertText(e,t){const n=this.getText(e);if(n!==t)throw new Error(`Expected '${e}' to contain '${t}', got '${n}'`);return this},assertAuthenticated(){if(!this.isAuthenticated())throw new Error("Expected user to be authenticated");return this},assertRole(e){if(!this.hasRole(e))throw new Error(`Expected user to have role '${e}'. Has: [${this.getRoles().join(", ")}]`);return this},getTransitions(){return[{id:"transfer",name:"Transfer",description:"Transfer tokens from sender to recipient"},{id:"approve",name:"Approve",description:"Approve spender to transfer tokens on owner's behalf"},{id:"transfer_from",name:"Transfer From",description:"Transfer tokens using allowance (delegated transfer)"},{id:"mint",name:"Mint",description:"Create new tokens and add to recipient balance"},{id:"burn",name:"Burn",description:"Destroy tokens from holder's balance"}]},getPlaces(){return[{id:"total_supply",name:"TotalSupply",initial:0},{id:"balances",name:"Balances",initial:0},{id:"allowances",name:"Allowances",initial:0}]},getTransition(e){return this.getTransitions().find(t=>t.id===e)||null},canFire(e){if(!this.getTransition(e))return{canFire:!1,reason:`Unknown transition: ${e}`};if(!s)return{canFire:!1,reason:"No current instance"};if(!this.isEnabled(e)){const a=this.getStatus();return{canFire:!1,reason:`Transition '${e}' not enabled in state '${a}'`,currentState:a,enabledTransitions:this.getEnabled()}}return{canFire:!0}},async sequence(e,t={}){const n=[],{stopOnError:a=!0,data:o={}}=t;for(const r of e){const l=this.canFire(r);if(!l.canFire){if(a)throw new Error(`Sequence failed at '${r}': ${l.reason}`);n.push({transition:r,success:!1,error:l.reason});continue}try{const i=await this.action(r,o[r]||{});n.push({transition:r,success:!0,state:i.state})}catch(i){if(a)throw i;n.push({transition:r,success:!1,error:i.message})}}return n},getWorkflowInfo(){var e;return{places:this.getPlaces(),transitions:this.getTransitions(),initialPlace:(e=this.getPlaces().find(t=>t.initial>0))==null?void 0:e.id}}};Qe();we();
