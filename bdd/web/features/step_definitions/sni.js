// SPDX-License-Identifier: AGPL-3.0-or-later
const {When,Then}=require('@cucumber/cucumber');
const assert=require('node:assert/strict');
When('I save spoof name {string}',async function(name){
 const field=await this.find('#spoof_sni');
 const details=await field.findElement(require('selenium-webdriver').By.xpath('ancestor::details'));
 if(!(await details.getAttribute('open'))) await details.findElement(require('selenium-webdriver').By.css('summary')).click();
 await field.clear();await field.sendKeys(name);
 await this.clickAndWaitForPageUpdate('form[action="/sni"] button[type="submit"]');
});
Then('the spoof name is {string}',async function(name){assert.equal(await (await this.find('#spoof_sni')).getAttribute('value'),name);});
