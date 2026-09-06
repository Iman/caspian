<div dir="rtl" align="right">

# التثبيت على macOS

<div dir="ltr" align="left">

[English](https://github.com/Iman/caspian/wiki/Install-macOS) | [فارسی](https://github.com/Iman/caspian/wiki/Install-macOS.fa) | [Русский](https://github.com/Iman/caspian/wiki/Install-macOS.ru) | [中文](https://github.com/Iman/caspian/wiki/Install-macOS.zh) | [العربية](https://github.com/Iman/caspian/wiki/Install-macOS.ar) | [اردو](https://github.com/Iman/caspian/wiki/Install-macOS.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Install-macOS.tr)

</div>

تحتاج إلى macOS 13 أو أحدث وحساب مسؤول. عند استخدام Wi-Fi المدمج كنقطة اتصال، وفّر اتصال إنترنت عبر Ethernet.

1. افتح صفحة الإصدارات الرسمية واختر DMG لمعالجك: <span dir="ltr">`arm64`</span> لأجهزة Apple Silicon أو <span dir="ltr">`amd64`</span> لأجهزة Intel.
2. افتح الملف واسحب <span dir="ltr">`Caspian.app`</span> إلى مجلد Applications، ثم افتح النسخة الموجودة فيه.
3. إذا ظهر تحذير بأن Apple لم تتمكن من التحقق من التطبيق، أغلقه بزر Done. بعد التحقق من مصدر الملف، افتح System Settings ثم Privacy & Security واختر Open Anyway بجانب Caspian.
4. اسمح بالتثبيت باستخدام كلمة مرور المسؤول واحتفظ بكلمة مرور اللوحة التي يعرضها الإعداد الأول.
5. انتظر Caspian is ready، ثم اختر Open panel وأدخل إعدادات Wi-Fi والوكيل.

إذا ظل الملف الخلفي <span dir="ltr">`caspian`</span> محظورًا رغم الموافقة على التطبيق، فقد تبقى عليه سمة الحجر. استخدم الأمر التالي فقط لتحذير المطور غير الموثّق أو التطبيق غير الموثّق لدى Apple، بعد التحقق من المصدر ومجموع التحقق.

إذا سمّى التحذير برنامج Trojan أو أبلغ عن برمجيات ضارة، أوقف التثبيت ولا تستخدم هذا الأمر. أبلغ عن نص التنبيه واسم الكشف ورقم الإصدار ورابط التنزيل.

<div dir="ltr" align="left">

```bash
sudo xattr -d com.apple.quarantine /usr/local/bin/caspian
```

</div>

ثم اختر Advanced options ثم Restart services. يزيل الأمر سمة الحجر من الملف المحدد فقط؛ ولا يفحص الملف أو يوقّعه. تعني رسالة <span dir="ltr">`No such xattr`</span> أن السمة غير موجودة. إذا استمر العطل، أبلغ عنه بدل إزالة ضوابط أمان أخرى.

كلمة مرور Mac وكلمة مرور اللوحة وكلمة مرور Wi-Fi منفصلة. عند نسيان كلمة مرور اللوحة، استخدم Reset password في Caspian Control مع صلاحيات المسؤول.

[التفاصيل بالإنجليزية](https://github.com/Iman/caspian/wiki/Installation#macos-13-or-later)

[ويكي Caspian](https://github.com/Iman/caspian/wiki/Home.ar)

<div dir="ltr" align="left">

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

</div>

</div>
