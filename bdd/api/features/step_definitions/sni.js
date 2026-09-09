// SPDX-License-Identifier: AGPL-3.0-or-later
const {When,Then}=require('@cucumber/cucumber');
const assert=require('node:assert/strict');
When('I save spoof name {string}',async function(name){const token=await this.tokenOn('/');await this.postForm('/sni',{csrf:token,spoof_sni:name});assert.equal(this.response.status,303);});
Then('the spoof name is {string}',async function(name){await this.get('/');const match=this.responseBody.match(/id="spoof_sni"[^>]*value="([^"]*)"/);assert.ok(match,'SNI field missing');assert.equal(match[1],name);});
