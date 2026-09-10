<!-- wiki-navigation:start -->
<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/SNI-Spoofing) · [فارسی](https://github.com/Iman/caspian/wiki/SNI-Spoofing.fa) · [Русский](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ru) · [简体中文](https://github.com/Iman/caspian/wiki/SNI-Spoofing.zh) · [**العربية**](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ar) · [Türkçe](https://github.com/Iman/caspian/wiki/SNI-Spoofing.tr) · [اردو](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ur)

</div>

<div dir="rtl" lang="ar">

[ويكي Caspian](https://github.com/Iman/caspian/wiki/Home.ar) · [استكشاف الأخطاء وإصلاحها](https://github.com/Iman/caspian/wiki/Troubleshooting.ar)

</div>
<!-- wiki-navigation:end -->

<div dir="rtl" align="right">

<a id="caspian-sni-spoofing-and-tls-splitting-for-dpi-circumvention"></a>
# خداع Caspian SNI وتقسيم TLS للتحايل على DPI

يغطي هذا الدليل التحايل على فحص الحزم العميق الاختياري (DPI) في `feature/sni`، وليس المثبت الذي تم إصداره حاليًا.

SNI هو اسم الخادم في تحية TLS.
ترسل هذه الميزة تحية إضافية باسم مزيف قبل دفق الوكيل الحقيقي.
يبقى اسم TLS أو REALITY الحقيقي في التكوين المستورد الخاص بك دون تغيير.
لا تعمل قيمة `sni` المستوردة على تمكين الانتحال تلقائيًا.

<a id="set-a-spoof-name"></a>
## تعيين اسم محاكاة ساخرة

1. احفظ تكوين الوكيل الخاص بك في اللوحة.
2. افتح **تحايل DPI (اختياري)** على لوحة القيادة.
3. أدخل اسم المجال، مثل `cover.example.invalid` للاختبار المحلي.
4. احفظ الإعداد.

استخدم نطاقًا مناسبًا لشبكتك الفعلية؛ لا يمكن حل مجال المثال.
إذا كان Caspian قيد التشغيل، فإن الحفظ يعيد توصيله بالإعداد الجديد.
لتعطيل الانتحال، قم بمسح الحقل وحفظه.
يؤدي استبدال التكوين إلى مسح الاسم المزيف وإعدادات التقسيم.
يؤدي تحديد إدخال آخر أو تحديث الاشتراك إلى الحفاظ عليه.

<a id="independent-tcp-split-and-tls-record-split"></a>
## تقسيم TCP مستقل وتقسيم سجل TLS

افتح **تحايل DPI (اختياري)** بجانب التكوين المحفوظ:

- **اسم الخادم المزيف**: اتركه فارغًا لإيقاف تشغيل SNI المزيف.
- **تقسيم TCP**: أرسل تحية TLS الأولية في كتابتين بالقرب من منتصف اسم مضيف SNI.
- **تقسيم سجل TLS**: قم بتقسيم سجل تحية TLS في هذا الموضع دون تغيير محتويات المصافحة.

كل خيار مستقل. يمكنك الجمع بينهما أو ترك الثلاثة.
بدون ملحق SNI، يستخدم التقسيم موضعًا بعد نوع المصافحة مباشرة.
يعد تقسيم TCP أفضل جهد: لا تضمن عمليات الكتابة المنفصلة وجود حزم منفصلة على كل نظام تشغيل وشبكة.
يؤدي تقسيم سجل TLS إلى تغيير حدود السجل ويمكن أن يفشل مع بعض الخوادم أو شبكات CDN.
يتطلب كلا الخيارين المقسمين حاليًا TLS عاديًا مع VLESS أو VMess أو Trojan عبر وسائل نقل IPv4 TCP المدعومة.
لم يتم تمكين تقسيم REALITY؛ لا يزال بإمكان REALITY استخدام SNI المزيف بمفرده.
يتم تقسيم ClientHello الأولي فقط؛ يتم ترحيل حركة مرور التطبيق لاحقًا بشكل طبيعي.
لا يحتاج وضع الانقسام فقط إلى الوصول إلى WinDivert أو مقبس الحزمة أو BPF.
عند تحديد SNI مزيف أيضًا، يجب أن ينتهي تأكيد الحزمة الخاصة به قبل إرسال أي تحية حقيقية.
تؤدي التحيات المشوهة أو غير المكتملة أو كبيرة الحجم أو المتوقفة إلى إغلاق الاتصال؛ لا تؤدي إعادة المحاولة إلى إزالة الخيار الممكّن بصمت.
يؤدي الحفظ إلى إعادة توصيل نفق نشط؛ يؤدي استبدال التكوين إلى مسح الخيارات الثلاثة جميعها.

حقول الحالة هي `spoof_sni`، و`tcp_split`، و`tls_record_split`.
تؤدي ترقية ملف الإصدار 4 إلى الحفاظ على اسمه المزيف وتترك إعدادات التقسيم معطلة.

<a id="state-compatibility"></a>
## توافق الدولة

يكتب هذا الفرع إصدار مخطط الحالة 5.
ترفض الإصدارات الأقدم ملف الحالة هذا لمنع فقدان البيانات.
احتفظ بنسخة احتياطية لحالة ما قبل الترقية إذا كنت بحاجة إلى العودة إلى إصدار أقدم.

<a id="limits"></a>
## حدود

يدعم هذا الإصدار VLESS وVMess وTrojan عبر IPv4 TCP، بما في ذلك WebSocket وHTTPUpgrade وgRPC.
وهو لا يدعم خوادم Hysteria2 أو QUIC أو SOCKS أو Shadowsocks أو XHTTP أو IPv6 فقط.
يمكن لـ SOCKS التفاوض على نقطة نهاية UDP منفصلة لا يستطيع معيد توجيه TCP تغطيتها.
لا يمكن لـ Shadowsocks UDP الأصلي، بما في ذلك حركة مرور DNS النفقية، استخدام معيد توجيه TCP فقط.
يختار معيد التوجيه أول عنوان IPv4 متاح من عناوين الخادم المكتشفة.
إذا فشل التأكيد، فسيتم إغلاق الاتصال دون إعادة توجيه الدفق الحقيقي.
لا يعود إلى الاتصال العادي.

يحتاج Fake SNI على نظام التشغيل Windows x64 إلى ملفات WinDivert المضمنة في بنية المثبت المحلي.
لا يمكن لـ Windows ARM64 استخدام SNI المزيف؛ لا يستخدم وضع الانقسام فقط الواجهة الخلفية للحزمة.
يحتاج SNI المزيف على Linux إلى امتيازات مأخذ الحزمة؛ يحتاج نظام التشغيل macOS إلى الوصول إلى جهاز BPF على واجهة بنمط Ethernet.
تمتلك خدمة Caspian المميزة هذه الموارد وتغلقها عندما تتوقف.

يثبت اختبار الاسترجاع لنظام التشغيل Windows أن الخادم يتلقى الدفق الحقيقي دون تغيير.
ولا يثبت أن الانتحال يعمل ضد تصفية مزود الخدمة الخاص بك.
تحتاج إصدارات Linux وmacOS أيضًا إلى اختبارات حزم مباشرة على أنظمتها المستهدفة.

<a id="credits"></a>
## الاعتمادات

المصدر والفكرة الأساسية هي [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing)، المرخصة بموجب GPL-3.0.
انظر [اعتمادات طرف ثالث](https://github.com/Iman/caspian/blob/feature/sni/docs/THIRD-PARTY.md).

<a id="dpi-bypass-sni-spoofing-and-security"></a>
## تجاوز DPI وانتحال SNI والأمن

<a id="is-caspian-dpi-safe"></a>
### هل Caspian DPI آمن؟

لا يوجد ضمان عالمي آمن لـ DPI. يحاول انتحال SNI الاختياري التأثير على كيفية قراءة نظام التصفية لحركة مرور TCP الأولية.
ولا يخفي عنوان IP للخادم أو حجم حركة المرور أو التوقيت، ولا يزال بإمكان المزود حظر الاتصال.
يحتفظ تطبيق `feature/sni` بهوية TLS الحقيقية ويرفض تأكيد المحاكاة الساخرة الفاشلة بدلاً من إرسال الدفق الحقيقي مباشرةً.

<a id="does-caspian-include-goodbyedpi-or-zapret"></a>
### هل يتضمن Caspian GoodbyeDPI أو zapret؟

رقم [GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI) و[zapret](https://github.com/bol-van/zapret) هما مرجعان بحثيان للاستراتيجيات المستقبلية المحتملة.
يأتي رمز SNI الأساسي والفكرة من [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing)، مع الحفاظ على إسناد GPL.
لا يقوم Caspian بتجميع تلك المشاريع الأخرى أو يدعي أن مؤلفيها يؤيدونها.

<a id="does-a-config-with-sni-enable-dpi-bypass-automatically"></a>
### هل يؤدي التكوين باستخدام SNI إلى تمكين تجاوز DPI تلقائيًا؟

لا. إن SNI المستورد هو هوية الخادم الحقيقية. قم بتعيين اسم محاكاة ساخرة اختياري منفصل لتمكين هذا الوضع.
اقرأ [إعداد SNI والقيود](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ar) قبل تمكينه.

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

[نتائج التحقق من الصحة واختبارات الأجهزة المتبقية](https://github.com/Iman/caspian/blob/feature/sni/docs/SNI-VALIDATION.md).

</div>

<!-- English-source-sha256: 0c3a086c7127a1bec0deacf690b2aed7696a340ce94ec55379e1f233bee629f0 -->
