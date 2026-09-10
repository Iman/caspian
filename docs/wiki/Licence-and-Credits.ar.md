<!-- wiki-navigation:start -->
<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Licence-and-Credits) · [فارسی](https://github.com/Iman/caspian/wiki/Licence-and-Credits.fa) · [Русский](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ru) · [简体中文](https://github.com/Iman/caspian/wiki/Licence-and-Credits.zh) · [**العربية**](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ar) · [Türkçe](https://github.com/Iman/caspian/wiki/Licence-and-Credits.tr) · [اردو](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ur)

</div>

<div dir="rtl" lang="ar">

[ويكي Caspian](https://github.com/Iman/caspian/wiki/Home.ar) · [استكشاف الأخطاء وإصلاحها](https://github.com/Iman/caspian/wiki/Troubleshooting.ar)

</div>
<!-- wiki-navigation:end -->

<div dir="rtl" align="right">

<a id="licence-and-credits"></a>
# الترخيص والاعتمادات

> يأتي هذا الدليل من ملف README الموجود. تحتفظ قياساتها بتواريخها الأصلية. لا يُبلغ نقل التوثيق هذا عن تشغيل اختباري جديد.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="licence"></a>
## الترخيص

AGPL-3.0-or-later، مع ثلاثة شروط إضافية بموجب القسم 7. الثلاثة جميعها من
يسمح القسم 7 اللطيف ولا شيء يقيد ما يمكنك فعله بالبرنامج:
الحفاظ على إشعار حقوق الطبع والنشر، وهذا الإسناد وإشارة واضحة إلى
مشروع قزوين في أي واجهة مستخدم؛ ضع علامة على الإصدار الخاص بك على أنه تم تغييره إذا كنت
تعديله؛ ولا تستخدم أسماء المؤلفين أو المشروع للدعاية،
والتي تتضمن التماس التبرعات أو الرعاية أو المنح بهذه الأسماء. ال
النص الكامل موجود في [`LICENSE`](https://github.com/Iman/caspian/blob/main/LICENSE) والمصطلحات موجودة في [`NOTICE`](https://github.com/Iman/caspian/blob/main/NOTICE).

يقيد هذا المصطلح الثالث استخدام NAMES ولا شيء آخر. تظل حراً في
تشغيل ودراسة وتعديل وإعادة توزيع البرنامج بموجب AGPL لأي شخص
غرض بما في ذلك غرض تجاري. ما لا يجوز لك فعله هو جمع الأموال في
اسم المؤلفين.

AGPL بدلاً من GPL، لأن هذا البرنامج يعمل عادةً كـ
الخدمة التي يتصل بها الآخرون، والقسم 13 يسد الفجوة في GPL العادي
أوراق. ليس ترخيصًا متساهلاً، لأن الثنائي يرتبط بشكل ثابت
رمز GPL-3.0-or-later: `github.com/sagernet/sing` و
`github.com/sagernet/sing-shadowsocks`، تم الوصول إليهما عبر نواة الأشعة السينية. لذلك
يجب أن يكون العمل المشترك وفق شروط عائلة GPL، ولا ينطبق ذلك على MIT أو Apache-2.0.
متاح لذلك.

<a id="built-on"></a>
## بنيت على

Caspian عبارة عن كمية صغيرة من التعليمات البرمجية حول عمل الآخرين. المحرك هو
xray-core، ومحلل ارتباط المشاركة هو XTLS. ولا يؤيد أي من المشروعين هذا
واحد؛ يُنسب إليهم الفضل لأن العمل هو عملهم.

| مشروع | الترخيص | ماذا يفعل هنا |
|---|---|---|
| [xray-core](https://github.com/xtls/xray-core) | MPL-2.0 | محرك الوكيل، مرتبط أثناء العملية بدلاً من تشغيله كبرنامج منفصل |
| [libXray](https://github.com/XTLS/libXray) | MIT | محلل ارتباط المشاركة، المباع تحت `third_party/libxray-share/` |
| [REALITY](https://github.com/xtls/reality) | MPL-2.0 | نقل التمويه TLS |
| [uTLS](https://github.com/refraction-networking/utls) | BSD-3-Clause | تقليد بصمة TLS |
| [quic-go](https://github.com/apernet/quic-go) | MIT | يتم تشغيل مكدس QUIC Hysteria2 |
| [gVisor](https://github.com/google/gvisor) | Apache-2.0 | تقوم شبكة مساحة المستخدمين بتكديس استخدامات TUN الواردة |
| [sing](https://github.com/sagernet/sing) و[sing-shadowsocks](https://github.com/sagernet/sing-shadowsocks) | GPL-3.0-or-later | Shadowsocks 2022، وسبب ترك هذا المشروع |
| [netlink](https://github.com/vishvananda/netlink) | Apache-2.0 | الواجهات والعناوين والطرق |
| [miekg/dns](https://github.com/miekg/dns) | BSD-3-Clause | التعامل مع رسائل DNS |
| [gorilla/websocket](https://github.com/gorilla/websocket) | BSD-2-Clause | النقل WebSocket |
| [CIRCL](https://github.com/cloudflare/circl) | BSD-3-Clause | تبادل المفاتيح بعد الكم |
| [Wintun](https://www.wintun.net/) | ترخيص Wintun للثنائيات المُصممة مسبقًا | برنامج تشغيل النفق `wintun.dll` الموقع على نظام التشغيل Windows |
| [وقت تشغيل .NET ونماذج Windows](https://github.com/dotnet/runtime) | MIT | مساعد Windows المستقل ووقت تشغيل تطبيق الدرج |
| `System.ServiceProcess.ServiceController` | MIT | التحكم في خدمة Windows من `CaspianControl.exe` |

يتضمن تثبيت Windows `wintun.dll`. يتضمن إصدار SNI أيضًا WinDivert على نظام التشغيل Windows x64.
يقوم Caspian بتوزيع Wintun 0.14.1 الثنائي الرسمي الموقع دون تغييرات.
ترخيصها في
[`third_party/wintun/PREBUILT-BINARIES-LICENSE.txt`](https://github.com/Iman/caspian/blob/main/third_party/wintun/PREBUILT-BINARIES-LICENSE.txt) ويتم نسخه إلى
`C:\Program Files\Caspian\WINTUN-LICENSE.txt` أثناء التثبيت.

`caspian-tethering.exe` و`CaspianControl.exe` هما .NET مستقلان بذاتهما
البرامج. مكونات .NET الخاصة بهم موجودة داخل الملفات القابلة للتنفيذ، وليس بجانبها
لهم كمكتبات DLL إضافية. ترخيص .NET والإشعارات موجودة في `third_party/dotnet/`.
تعد الحزمة المرجعية لـ Windows SDK عبارة عن إدخال بناء ولم يتم تثبيتها معها
قزوين.

كما أنها تحتاج إلى `hostapd` و`dnsmasq` و`nftables` و`iw` و`iproute2` على
آلة. يتم تشغيل هذه البرامج كبرامج منفصلة بدلاً من ربطها ببعضها البعض
التراخيص لا تؤثر على هذا، ولكن الجهاز لا شيء بدونها.

[`NOTICE`](https://github.com/Iman/caspian/blob/main/NOTICE) يحمل السجل الكامل: كل وحدة في الثنائي، قراءة الترخيص
من ملف الترخيص الخاص به، وأسباب التوافق.

<!-- SNI upstream credits -->

أرصدة انتحال SNI: [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing) (GPL-3.0)، مع WinDivert (LGPL-3.0) على نظام التشغيل Windows x64.
[تراخيص الطرف الثالث، والإصدارات المصدر، والائتمانات](https://github.com/Iman/caspian/blob/feature/sni/docs/THIRD-PARTY.md).

<a id="sni-idea-acknowledgements"></a>
## اعترافات فكرة SNI

ينسب Caspian أيضًا الفضل إلى مؤلفي هذه المشاريع والمساهمين فيها للأفكار ومقارنات التنفيذ التي استرشد بها عمل SNI.
لا يتم تجميع التعليمات البرمجية والملفات التنفيذية الخاصة بهم.

- [selfishblackberry177/sni-spoof](https://github.com/selfishblackberry177/sni-spoof): مقارنة إعادة توجيه Go SNI.
- [therealaleph/sni-spoofing-rust](https://github.com/therealaleph/sni-spoofing-rust): تنفيذ Rust SNI ومقارنة الميزات.
- [bol-van/zapret](https://github.com/bol-van/zapret): استراتيجيات وتشخيصات التحايل على DPI.
- [ValdikSS/GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI): استراتيجيات التحايل على DPI.
- [Floxu1/UAC-SNI-Spoofer-Android](https://github.com/Floxu1/UAC-SNI-Spoofer-Android): أفكار تكامل Android SNI.

يحتفظ الكود المعدل بترخيصه وإشعاراته الأولية.
لا تمنح إقرارات الفكرة الإذن بنسخ التعليمات البرمجية أو تشير ضمنيًا إلى التأييد.
راجع [اعتمادات طرف ثالث](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ar) للاطلاع على الإصدارات والتراخيص ونطاق الاستخدام الذي تمت مراجعته.

[WinDivert — باسل (basil00)](https://github.com/basil00/WinDivert/tree/v2.2.2): نظام التشغيل Windows x64، LGPL-3.0.

</div>

<!-- English-source-sha256: 626ed23e3eab55bb351c5d12fc420fb06471c2cf6e2e08fb42edb6a8062571bf -->
