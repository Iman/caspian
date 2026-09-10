// SPDX-License-Identifier: AGPL-3.0-or-later
const {Given,When,Then}=require('@cucumber/cucumber');
const assert=require('node:assert/strict');
When('I save spoof name {string}',async function(name){
 const field=await this.find('#spoof_sni');
 const details=await field.findElement(require('selenium-webdriver').By.xpath('ancestor::details'));
 if(!(await details.getAttribute('open'))) await details.findElement(require('selenium-webdriver').By.css('summary')).click();
 await field.clear();await field.sendKeys(name);
 await this.clickAndWaitForPageUpdate('form[action="/sni"] button[type="submit"]');
});
Then('the spoof name is {string}',async function(name){assert.equal(await (await this.find('#spoof_sni')).getAttribute('value'),name);});

Given('a TLS config is stored for splitting',async function(){
 const field=await this.find('#config');const By=require('selenium-webdriver').By;
 const details=await field.findElement(By.xpath('ancestor::details'));if(!(await details.getAttribute('open')))await details.findElement(By.css('summary')).click();
 await field.clear();await field.sendKeys('vless://11111111-2222-3333-4444-555555555555@example.invalid:443?security=tls&type=tcp&sni=example.com');
 await this.clickAndWaitForPageUpdate('form[action="/config"] button[type="submit"]');
});
When('I save DPI options {string} TCP {string} TLS records {string}',async function(name,tcp,tls){
 const field=await this.find('#spoof_sni');const By=require('selenium-webdriver').By;
 const details=await field.findElement(By.xpath('ancestor::details'));if(!(await details.getAttribute('open')))await details.findElement(By.css('summary')).click();
 await field.clear();await field.sendKeys(name);
 for(const [key,value] of [['tcp_split',tcp],['tls_record_split',tls]]){const box=await this.find('input[name="'+key+'"]');if(await box.isSelected()!==(value==='on'))await box.click();}
 await this.clickAndWaitForPageUpdate('form[action="/sni"] button[type="submit"]');
});
Then('TCP split is {string} and TLS-record split is {string}',async function(tcp,tls){
 for(const [key,value] of [['tcp_split',tcp],['tls_record_split',tls]])assert.equal(await (await this.find('input[name="'+key+'"]')).isSelected(),value==='on');
});
