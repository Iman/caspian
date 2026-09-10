<!-- wiki-navigation:start -->
<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Development-and-Testing) · [فارسی](https://github.com/Iman/caspian/wiki/Development-and-Testing.fa) · [Русский](https://github.com/Iman/caspian/wiki/Development-and-Testing.ru) · [简体中文](https://github.com/Iman/caspian/wiki/Development-and-Testing.zh) · [**العربية**](https://github.com/Iman/caspian/wiki/Development-and-Testing.ar) · [Türkçe](https://github.com/Iman/caspian/wiki/Development-and-Testing.tr) · [اردو](https://github.com/Iman/caspian/wiki/Development-and-Testing.ur)

</div>

<div dir="rtl" lang="ar">

[ويكي Caspian](https://github.com/Iman/caspian/wiki/Home.ar) · [استكشاف الأخطاء وإصلاحها](https://github.com/Iman/caspian/wiki/Troubleshooting.ar)

</div>
<!-- wiki-navigation:end -->

<div dir="rtl" align="right">

<a id="development-and-testing"></a>
# التطوير والاختبار

> يأتي هذا الدليل من ملف README الموجود. تحتفظ قياساتها بتواريخها الأصلية. لا يُبلغ نقل التوثيق هذا عن تشغيل اختباري جديد.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="running-it"></a>
## تشغيله

قم ببناء الملف الثنائي وسلمه إلى المثبت. هذا المسار لا يحتاج إلى إطلاق، و
يأخذها المثبت للتثبيت الحقيقي بالإضافة إلى التشغيل الجاف:

انتقل إلى البناء -o /tmp/caspian-linux-arm64 ./cmd/caspian
sha256sum /tmp/caspian-linux-arm64 | سيد 's|/tmp/||' > /tmp/SHA256SUMS

env CASPIAN_LOCAL_BINARY=/tmp/caspian-linux-arm64 \
CASPIAN_LOCAL_CHECKSUMS=/tmp/SHA256SUMS \
bash install.sh --dry-run --yes

قم بإسقاط `--dry-run` للتثبيت بشكل حقيقي. بدون `CASPIAN_LOCAL_CHECKSUMS`
يحذر المثبت، بهذه الكلمات، من أنه يقوم بتثبيت برنامج ثنائي لم يتم التحقق منه.
[`docs/INSTALL.md`](https://github.com/Iman/caspian/blob/main/docs/INSTALL.md) هو دليل التشغيل الكامل. يتضمن حزام `uname` المزيف
المشي الرفض على الجهاز الذي لا يمكن تثبيته.

يحتوي الثنائي على أربعة أوامر فرعية:

خدمة قزوين - الجذر المميز: الطرق، جدار الحماية، نقطة الوصول، المحرك
خدمة قزوين - لوحة مستخدم قزوين: لوحة الويب، لا يوجد شيء مميز
قزوين تحقق من تقرير كيف يبدو هذا المربع؛ لا يغير شيئا
نسخة قزوين

لا يوجد أي أمر فرعي يطبق التكوين أو يحرك المفتاح بشكل متعمد.
تقول واجهة سطر الأوامر (CLI) ذلك بنفسها: "بعد تشغيل برنامج التثبيت، كل ما يفعله الشخص
يحدث في اللوحة."

يقوم [`uninstall.sh`](https://github.com/Iman/caspian/blob/main/uninstall.sh) بإزالة الوحدات والثنائيات والأدلة وإعادة التشغيل
مجلة الشبكة بحيث يتم ترك الصندوق كما تم العثور عليه. اقرأ [العيب د5](https://github.com/Iman/caspian/wiki/Troubleshooting.ar) قبل أن تعتمد عليه.

<a id="the-rules-this-project-holds-itself-to"></a>
## القواعد التي يحملها هذا المشروع لنفسه

هذه ليست تطلعات. ولكل واحد آلية، والآلية مسماة.

**لا يوجد شيء يسمى العمل دون الحصول على عنوان IP للخروج من حركة المرور الحقيقية.**
[`docs/2026-08-29-design.md`](https://github.com/Iman/caspian/blob/main/docs/2026-08-29-design.md)، القسم 6. الاتصال ليس نتيجة. الأجهزة
درجات تسخير UNPROVEN، وليس PASS، عندما لم يتم التقاط أي مخرج IP، ويتم الخروج 1.

**جملة خاطئة واثقة أسوأ من عدم وجود جملة.** القارئ الذي يقال له
يتم التعامل مع شيء ما بشكل صحيح ويخلص إلى أنه لا يوجد شيء للتحقق. لذلك أ
التصحيح يترك اختبارًا خلفه بدلاً من جملة أفضل.
`TestNothingInTheApplianceWatchesTheUplink` موجود بسبب وثيقتين مرة واحدة
ادعى أن الصندوق يراقب الوصلة الصاعدة الخاصة به ويعيد تحميل جدار الحماية عندما يتحرك.

**البدء في العملية ليس دليلاً على نجاحها.** واجهة نقطة الاتصال هي كذلك
اقرأ مرة أخرى من النواة قبل أن يرتبط أي شيء بها، وتكون نقطة الوصول كذلك
أعد القراءة قبل أن تعلن الخدمة عن تشغيلها. تمت إضافة كلا القراءتين
بعد حدث واحد مُقاس حقق فيه كل أمر النجاح.

**تمت مشاهدة كل سيناريو وهو يفشل.** يقوم `TestEveryScenarioCanFail` بإدخال أ
عيب مسمى في كل سلوك ويتطلب أن يتحول إلى اللون الأحمر. اختبار لا أحد لديه
إن رؤية الفشل هو ضوء أخضر موصل إلى لا شيء.

**مصدر التثبيت موجود في اسم الملف الخاص به.** `capture-pi5-` هو بايت
لإخراج أمر حقيقي على الهدف، `scenario-` هو جهاز لا يملكه أحد
تم قياسها، و`golden-` هو مخرجات هذا المشروع. اختبار القراءة أ
يقدم الملف `capture-pi5-` مطالبة بشأن الهدف. اختبار قراءة `scenario-`
الملف لا.

**بيانات الاعتماد في الالتزام دائمة.** `test/goldenscan` يكتسح كل
تركيبات مخصصة للحراس المسجلين ولأشكال بيانات الاعتماد، وذلك
يتحقق من أسماء الملفات وكذلك نصوص الملفات. وقد شوهد اصطياد زرعة
سر كل فئة يعرفها.

**أرضيات التغطية عبارة عن سقاطة.** كل رقم في [`scripts/gate.sh`](https://github.com/Iman/caspian/blob/main/scripts/gate.sh) هو ما
حزمة تم قياسها بعد العمل الذي قدمتها، وليس هدفًا لشخص ما
يأمل. الحزمة التي لا تحتوي على صف ليست مسورة، وغياب الصف يعني
"لم يتم الاتفاق على حد أدنى بعد" بدلا من "تم تغطية هذه الحزمة".

**الجانب المميز لا يثق بأي شيء يرسله المتصل.** كل حقل من كل
يتم فحص الطلب مقابل ما اكتشفه هذا الجهاز بنفسه. الرفض هو أ
رمز الخطأ من مجموعة مغلقة، وليس جملة، ولا قيمة للمتصل أبدًا
أرسلت.

**يطلب الصندوق من الإنترنت شيئًا لم تطلبه منه.** لا يوجد قياس عن بعد، ولا هاتف منزلي، ولا تعطل
تحميل، ولا يوجد خط ويب، ولا يوجد ملف بيانات جغرافية، ولا يوجد محلل Google بشكل افتراضي.

[Architecture](https://github.com/Iman/caspian/wiki/Architecture.ar) | [Panel-and-Configuration](https://github.com/Iman/caspian/wiki/Panel-and-Configuration.ar) | [Troubleshooting](https://github.com/Iman/caspian/wiki/Troubleshooting.ar)

</div>

<!-- English-source-sha256: 5badcd2d45aa3aa7f927a0215bc4511dcd899932a2c648c8bec2484f9c77a161 -->
