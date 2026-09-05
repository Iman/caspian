// SPDX-License-Identifier: AGPL-3.0-or-later
// Run with: node --test internal/panel/js_tests/panel_async_test.cjs
const {test} = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');

function harness() {
  const requests = [], timers = new Map();
  let timerID = 0, replaced = 0;
  const feedback = {hidden:true, dataset:{working:'working', stopping:'stopping', failed:'unknown result', stoppedRemote:'disconnected after stop', stale:'stale'}};
  const buttons = [{disabled:false}, {disabled:true}];
  const doc = {
    getElementById: id => id === 'action-feedback' ? feedback : null,
    querySelectorAll: selector => selector.includes('submit') ? buttons : [],
    body:{replaceWith() {replaced++;}},
    importNode: x => x, documentElement:{},
  };
  const window = {fetch:true, AbortController, location:{href:'http://127.0.0.1:8088/', origin:'http://127.0.0.1:8088'},
    setTimeout(fn) {timers.set(++timerID, fn);return timerID;}, clearTimeout(id) {timers.delete(id);}, history:{replaceState(){}}};
  const fetch = (url, options) => new Promise((resolve, reject) => {
    requests.push({url,options,resolve,reject});
    options.signal.addEventListener('abort',()=>reject(new Error('aborted')));
  });
  const page = {getElementById:()=>({}), body:{}, title:'Caspian', documentElement:{lang:'en',dir:'ltr'}};
  vm.runInNewContext(fs.readFileSync(path.join(__dirname,'../assets/panel.js'),'utf8'), {
    window,document:doc,fetch,AbortController,URL,URLSearchParams,
    FormData:class {constructor(form) {return new Map(form.values);}},
    DOMParser:class {parseFromString() {return page;}}
  });
  function submit(action='/power',values=[['csrf','token'],['on','0']]) {
    const form={method:'post',action:'http://127.0.0.1:8088'+action,values,setAttribute(){},removeAttribute(){}};
    doc.onsubmit({target:form,preventDefault(){}});
  }
  return {submit, requests, feedback, buttons, timers, replaced:()=>replaced};
}
const settle = () => new Promise(resolve=>setImmediate(resolve));

test('async forms capture CSRF and action, reject duplicate submission, render response', async () => {
  const h=harness(); h.submit(); h.submit();
  assert.equal(h.requests.length,1);
  assert.equal(h.requests[0].options.body.get('csrf'),'token');
  assert.equal(h.requests[0].options.body.get('on'),'0');
  assert.equal(h.buttons[0].disabled,true);
  assert.equal(h.feedback.textContent,'stopping');
  assert.equal(h.replaced(),0);
  h.requests[0].resolve({url:'http://127.0.0.1:8088/',headers:{get:()=> 'text/html'},text:async()=>'<main/>'});
  await settle();
  assert.equal(h.replaced(),1);
  assert.equal(h.timers.size,0);
});

test('deadline shows an unknown outcome and never retries a mutation', async () => {
  const h=harness(); h.submit('/hotspot');
  [...h.timers.values()][0](); await settle();
  assert.equal(h.feedback.textContent,'unknown result');
  assert.equal(h.buttons[0].disabled,false);
  assert.equal(h.buttons[1].disabled,true);
  assert.equal(h.requests.length,1);
  assert.equal(h.replaced(),0);
});

test('losing the portal while stopping explains host-side recovery without claiming success', async () => {
  const h=harness();h.submit();h.requests[0].reject(new Error('network lost'));await settle();
  assert.equal(h.feedback.textContent,'disconnected after stop');
  assert.equal(h.buttons[0].disabled,false);
});

function identifierHarness() {
  const requests = [], timers = new Map();
  let timerID = 0, reloads = 0, copies = 0;
  const status = {hidden:true,dataset:{working:'working',ready:'ready',failed:'failed',copied:'copied'}};
  const uuid = {value:'',focus(){},select(){},setSelectionRange(){}};
  const imei = {value:'',focus(){},select(){},setSelectionRange(){}};
  const generate = {disabled:false};
  const copyUUID = {disabled:true,dataset:{copyTarget:'generated-uuid'}};
  const copyIMEI = {disabled:true,dataset:{copyTarget:'generated-imei'}};
  const form = {method:'get',action:'http://127.0.0.1:8088/identifiers.json',setAttribute(){},removeAttribute(){}};
  const elements = {'identifier-generator':form,'generate-identifiers':generate,
    'identifier-status':status,'generated-uuid':uuid,'generated-imei':imei};
  const doc = {
    getElementById: id => elements[id] || null,
    querySelectorAll: selector => selector === '[data-copy-target]' ? [copyUUID,copyIMEI] : [],
    execCommand(command) {assert.equal(command,'copy');copies++;return true;}
  };
  const window = {fetch:true,AbortController,navigator:{},isSecureContext:false,
    location:{href:'http://127.0.0.1:8088/',origin:'http://127.0.0.1:8088',reload(){reloads++;}},
    setTimeout(fn) {timers.set(++timerID,fn);return timerID;},clearTimeout(id){timers.delete(id);}};
  const fetch = (url,options) => new Promise((resolve,reject) => {
    requests.push({url,options,resolve,reject});
    options.signal.addEventListener('abort',()=>reject(new Error('aborted')));
  });
  vm.runInNewContext(fs.readFileSync(path.join(__dirname,'../assets/panel.js'),'utf8'), {
    window,document:doc,fetch,AbortController,URL,URLSearchParams,
    FormData:class {},DOMParser:class {}
  });
  function submit() {form.onsubmit({preventDefault(){}});}
  return {submit,requests,timers,status,uuid,imei,generate,copyUUID,copyIMEI,
    reloads:()=>reloads,copies:()=>copies};
}

test('identifier generation is same-origin, asynchronous, validated and copyable on HTTP', async () => {
  const h=identifierHarness();h.submit();h.submit();
  assert.equal(h.requests.length,1);
  assert.equal(h.requests[0].url,'http://127.0.0.1:8088/identifiers.json');
  assert.equal(h.requests[0].options.credentials,'same-origin');
  assert.equal(h.requests[0].options.cache,'no-store');
  assert.equal(h.generate.disabled,true);
  assert.equal(h.status.textContent,'working');
  h.requests[0].resolve({status:200,ok:true,url:h.requests[0].url,
    headers:{get:()=> 'application/json'},json:async()=>({
      uuid:'12345678-1234-4123-8123-123456789abc',imei:'490154203237518'
    })});
  await settle();
  assert.equal(h.uuid.value,'12345678-1234-4123-8123-123456789abc');
  assert.equal(h.imei.value,'490154203237518');
  assert.equal(h.status.textContent,'ready');
  assert.equal(h.generate.disabled,false);
  assert.equal(h.copyUUID.disabled,false);
  h.copyUUID.onclick();
  assert.equal(h.copies(),1);
  assert.equal(h.status.textContent,'copied');
});

test('identifier deadline reports failure without retrying or hanging the control', async () => {
  const h=identifierHarness();h.submit();
  [...h.timers.values()][0]();
  await settle();
  assert.equal(h.requests.length,1);
  assert.equal(h.status.textContent,'failed');
  assert.equal(h.generate.disabled,false);
});
