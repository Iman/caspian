<!-- wiki-navigation:start -->
<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Third-Party-Credits) · [فارسی](https://github.com/Iman/caspian/wiki/Third-Party-Credits.fa) · [Русский](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ru) · [简体中文](https://github.com/Iman/caspian/wiki/Third-Party-Credits.zh) · [**العربية**](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ar) · [Türkçe](https://github.com/Iman/caspian/wiki/Third-Party-Credits.tr) · [اردو](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ur)

</div>

<div dir="rtl" lang="ar">

[ويكي Caspian](https://github.com/Iman/caspian/wiki/Home.ar) · [استكشاف الأخطاء وإصلاحها](https://github.com/Iman/caspian/wiki/Troubleshooting.ar)

</div>
<!-- wiki-navigation:end -->

<div dir="rtl" align="right">

<a id="third-party-code-and-credits"></a>
# رمز الطرف الثالث والائتمانات

يسرد [NOTICE](https://github.com/Iman/caspian/blob/feature/sni/NOTICE) مكتبات Caspian المرتبطة وملفات التوزيع.
يحتفظ كل مكون من مكونات المنبع بترخيصه الخاص.
لا تحل شروط AGPL الخاصة بـ Caspian محل تلك الإشعارات الأولية.

<a id="sni-spoofing"></a>
## انتحال SNI

الكود الأساسي والفكرة تأتي من **[patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing)**.
قام Caspian بمراجعة الالتزام `13b78cf7e073f38d9cadcff542faf4a00b0a6de2`.
يقوم قالب ClientHello وخوارزمية المصافحة بإبلاغ `internal/snispoof`.
تضيف تغييرات Caspian تكامل Go والتحقق من الصحة وملكية الاتصال وحدود الموارد والتراجع والاختبارات.

تحتفظ الملفات المصدر المشتقة بإشعارات GPL-3.0-only.
يظل [ترخيص GPL المنبع](https://github.com/Iman/caspian/blob/feature/sni/third_party/sni-spoofing/LICENSE.txt) و[نسبة العمل إلى أصحابه](https://github.com/Iman/caspian/blob/feature/sni/third_party/sni-spoofing/README.md) كاملين في المستودع.
يسمح القسم 13 من GPLv3 بالدمج مع كود AGPLv3 بينما يحتفظ كل جزء بشروطه الخاصة.
يجب على الموزعين الحفاظ على الإشعارات ووضع علامة على التغييرات وتوفير المصدر المقابل بموجب التراخيص المعمول بها.
الائتمان لا يعني موافقة المؤلفين المنبع.

يستخدم Windows x64 **[WinDivert](https://github.com/basil00/WinDivert/tree/v2.2.2)** بواسطة Basil (basil00) والمساهمين.
يختار Caspian LGPL-3.0 من رخصته المزدوجة.
يحتوي برنامج التثبيت على برنامج التشغيل غير المعدل وDLL وحزمة الترخيص الكاملة والإسناد وأرشيف المصدر للإصدار 2.2.2.
انظر [تفاصيل توزيع WinDivert](https://github.com/Iman/caspian/blob/feature/sni/third_party/windivert/README.md).
لا يتضمن Windows ARM64 WinDivert ولا يمكنه استخدام ميزة SNI هذه.

<a id="ideas-and-acknowledgements"></a>
## الأفكار والاعترافات

ينسب Caspian أيضًا الفضل إلى مؤلفي هذه المشاريع والمساهمين فيها للأفكار ومقارنات التنفيذ التي استرشد بها عمل SNI.
لا يتم تجميع التعليمات البرمجية والملفات التنفيذية الخاصة بهم.

| مشروع | المراجعة المراجعة | الترخيص والاستخدام |
| --- | --- | --- |
| [selfishblackberry177/sni-spoof](https://github.com/selfishblackberry177/sni-spoof) | `a0d68c6627e0898cf13176e322420743f69d894f` | لم يتم العثور على ملف الترخيص؛ المقارنة فقط |
| [ValdikSS/GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI) | `f593a276f9ec753889f80208c6a7c5cf455df94a` | Apache-2.0; مرجع الاستراتيجية |
| [bol-van/zapret](https://github.com/bol-van/zapret) | `87e058624c72863db53bdaf7fb6f16576dddb6ab` | MIT; الاستراتيجية والمرجع التشخيصي |
| [Floxu1/UAC-SNI-Spoofer-Android](https://github.com/Floxu1/UAC-SNI-Spoofer-Android) | `c68e350f5f9e9cff308c289db261d7c5d034efa2` | لم يتم العثور على ترخيص تطبيق عالي المستوى؛ الأفكار فقط |
| [therealaleph/sni-spoofing-rust](https://github.com/therealaleph/sni-spoofing-rust) | `d2956025c31d96f0f0a341af4f1a8eda204857c7` | تعلن MIT؛ مصدر قالب GPL يحتاج إلى توضيح؛ لم يتم نسخ أي رمز |

<a id="other-distributed-components"></a>
## المكونات الموزعة الأخرى

يحتفظ [محلل ارتباط المشاركة](https://github.com/Iman/caspian/blob/feature/sni/third_party/libxray-share/LICENSE) بترخيص MIT الخاص به.
تحتوي مثبتات Windows أيضًا على ثنائيات Wintun الرسمية ومساعدي .NET المستقلين.
تظل إشعاراتهم ضمن [third_party](https://github.com/Iman/caspian/blob/feature/sni/third_party) ويتم تثبيتها بجانب التطبيق.
يعد ربط Go Wintun أحد تبعيات وقت التشغيل MIT على Windows.

</div>

<!-- English-source-sha256: ceef83c2b7b0779eb04c1fa1c854f35aaf685978f4d17f439ddaa2fd2d01847c -->
