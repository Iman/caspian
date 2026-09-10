<!-- wiki-navigation:start -->
<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Third-Party-Credits) · [فارسی](https://github.com/Iman/caspian/wiki/Third-Party-Credits.fa) · [Русский](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ru) · [简体中文](https://github.com/Iman/caspian/wiki/Third-Party-Credits.zh) · [العربية](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ar) · [Türkçe](https://github.com/Iman/caspian/wiki/Third-Party-Credits.tr) · [**اردو**](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ur)

</div>

<div dir="rtl" lang="ur">

[Caspian ویکی](https://github.com/Iman/caspian/wiki/Home.ur) · [مسائل کا حل](https://github.com/Iman/caspian/wiki/Troubleshooting.ur)

</div>
<!-- wiki-navigation:end -->

<div dir="rtl" align="right">

<a id="third-party-code-and-credits"></a>
# فریق ثالث کوڈ اور کریڈٹس

[NOTICE](https://github.com/Iman/caspian/blob/feature/sni/NOTICE) Caspian کی منسلک لائبریریوں اور تقسیم کی فائلوں کی فہرست دیتا ہے۔
ہر اپ اسٹریم جزو اپنا لائسنس رکھتا ہے۔
Caspian کی AGPL شرائط ان اپ اسٹریم نوٹسز کی جگہ نہیں لیتی ہیں۔

<a id="sni-spoofing"></a>
## SNI جعل سازی

بنیادی کوڈ اور خیال **[patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing)** سے آتا ہے۔
Caspian کا جائزہ لیا کمٹ `13b78cf7e073f38d9cadcff542faf4a00b0a6de2`۔
ClientHello ٹیمپلیٹ اور ہینڈ شیک الگورتھم `internal/snispoof` کو مطلع کرتے ہیں۔
Caspian کی تبدیلیوں میں گو انٹیگریشن، توثیق، کنکشن کی ملکیت، وسائل کی حدود، رول بیک اور ٹیسٹ شامل ہیں۔

اخذ کردہ سورس فائلز GPL-3.0-only نوٹسز کو برقرار رکھتی ہیں۔
مکمل [اپ اسٹریم GPL لائسنس](https://github.com/Iman/caspian/blob/feature/sni/third_party/sni-spoofing/LICENSE.txt) اور [اعترافِ خدمات](https://github.com/Iman/caspian/blob/feature/sni/third_party/sni-spoofing/README.md) ریپوزٹری میں باقی ہیں۔
GPLv3 سیکشن 13 AGPLv3 کوڈ کے ساتھ امتزاج کی اجازت دیتا ہے جبکہ ہر حصہ اپنی شرائط کو برقرار رکھتا ہے۔
تقسیم کاروں کو قابل اطلاق لائسنس کے تحت نوٹس کو محفوظ کرنا، تبدیلیوں کو نشان زد کرنا اور متعلقہ ذریعہ فراہم کرنا چاہیے۔
کریڈٹ کا مطلب اپ اسٹریم مصنفین کی توثیق نہیں ہے۔

Windows x64 استعمال کرتا ہے **[WinDivert](https://github.com/basil00/WinDivert/tree/v2.2.2)** بذریعہ Basil (basil00) اور شراکت دار۔
Caspian اپنے دوہری لائسنس سے LGPL-3.0 کو منتخب کرتا ہے۔
انسٹالر میں غیر ترمیم شدہ ڈرائیور اور DLL، مکمل لائسنس بنڈل، انتساب، اور v2.2.2 کے لیے سورس آرکائیو شامل ہے۔
[WinDivert تقسیم کی تفصیلات](https://github.com/Iman/caspian/blob/feature/sni/third_party/windivert/README.md) دیکھیں۔
Windows ARM64 میں WinDivert شامل نہیں ہے اور یہ SNI خصوصیت استعمال نہیں کر سکتا۔

<a id="ideas-and-acknowledgements"></a>
## خیالات اور اعترافات

Caspian ان منصوبوں کے مصنفین اور تعاون کنندگان کو آئیڈیاز اور ان پر عمل درآمد کے موازنہ کا سہرا بھی دیتا ہے جنہوں نے اس کے ایس این آئی کے کام سے آگاہ کیا۔
ان کا کوڈ اور ایگزیکیوٹیبل بنڈل نہیں ہیں۔

| پروجیکٹ | نظرثانی کا جائزہ لیا۔ | لائسنس اور استعمال |
| --- | --- | --- |
| [selfishblackberry177/sni-spoof](https://github.com/selfishblackberry177/sni-spoof) | `a0d68c6627e0898cf13176e322420743f69d894f` | کوئی لائسنس فائل نہیں ملی؛ صرف موازنہ |
| [ValdikSS/GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI) | `f593a276f9ec753889f80208c6a7c5cf455df94a` | Apache-2.0; حکمت عملی کا حوالہ |
| [bol-van/zapret](https://github.com/bol-van/zapret) | `87e058624c72863db53bdaf7fb6f16576dddb6ab` | MIT; حکمت عملی اور تشخیصی حوالہ |
| [Floxu1/UAC-SNI-Spoofer-Android](https://github.com/Floxu1/UAC-SNI-Spoofer-Android) | `c68e350f5f9e9cff308c289db261d7c5d034efa2` | کوئی اعلیٰ سطحی ایپ لائسنس نہیں ملا؛ صرف خیالات |
| [therealaleph/sni-spoofing-rust](https://github.com/therealaleph/sni-spoofing-rust) | `d2956025c31d96f0f0a341af4f1a8eda204857c7` | MIT کا اعلان کرتا ہے؛ GPL ٹیمپلیٹ پرووننس وضاحت کی ضرورت ہے؛ کوئی کوڈ کاپی نہیں کیا گیا۔ |

<a id="other-distributed-components"></a>
## دیگر تقسیم شدہ اجزاء

[شیئر لنک پارسر](https://github.com/Iman/caspian/blob/feature/sni/third_party/libxray-share/LICENSE) نے اپنا MIT لائسنس برقرار رکھا ہے۔
ونڈوز انسٹالرز میں آفیشل ونٹن بائنریز اور خود ساختہ .NET مددگار بھی ہوتے ہیں۔
ان کے نوٹسز [third_party](https://github.com/Iman/caspian/blob/feature/sni/third_party) کے تحت رہتے ہیں اور درخواست کے ساتھ نصب ہیں۔
گو ونٹن بائنڈنگ ایک MIT ونڈوز پر رن ٹائم انحصار ہے۔

</div>

<!-- English-source-sha256: ceef83c2b7b0779eb04c1fa1c854f35aaf685978f4d17f439ddaa2fd2d01847c -->
