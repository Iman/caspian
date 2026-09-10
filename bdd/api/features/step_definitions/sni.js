// SPDX-License-Identifier: AGPL-3.0-or-later
const {Given,When,Then}=require('@cucumber/cucumber');
const assert=require('node:assert/strict');
When('I save spoof name {string}',async function(name){const token=await this.tokenOn('/');await this.postForm('/sni',{csrf:token,spoof_sni:name});assert.equal(this.response.status,303);});
Then('the spoof name is {string}',async function(name){await this.get('/');const match=this.responseBody.match(/id="spoof_sni"[^>]*value="([^"]*)"/);assert.ok(match,'SNI field missing');assert.equal(match[1],name);});

Given('a TLS config is stored for splitting',async function(){
 await this.postForm('/config',{csrf:await this.tokenOn('/'),config:'vless://11111111-2222-3333-4444-555555555555@example.invalid:443?security=tls&type=tcp&sni=example.com'});
 assert.equal(this.response.status,303);
});
When('I save DPI options {string} TCP {string} TLS records {string}',async function(name,tcp,tls){
 const form={csrf:await this.tokenOn('/'),spoof_sni:name};if(tcp==='on')form.tcp_split='on';if(tls==='on')form.tls_record_split='on';
 await this.postForm('/sni',form);assert.equal(this.response.status,303);
});
Then('TCP split is {string} and TLS-record split is {string}',async function(tcp,tls){
 await this.get('/');for(const [field,value] of [['tcp_split',tcp],['tls_record_split',tls]]){
  const match=this.responseBody.match(new RegExp('<input[^>]*name="'+field+'"[^>]*>'));assert.ok(match,'missing '+field);assert.equal(/\bchecked\b/.test(match[0]),value==='on');
 }
});
