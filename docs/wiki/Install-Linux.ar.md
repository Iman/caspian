<div dir="rtl" align="right">

# التثبيت على Linux وRaspberry Pi

<div dir="ltr" align="left">

[English](https://github.com/Iman/caspian/wiki/Install-Linux) | [فارسی](https://github.com/Iman/caspian/wiki/Install-Linux.fa) | [Русский](https://github.com/Iman/caspian/wiki/Install-Linux.ru) | [中文](https://github.com/Iman/caspian/wiki/Install-Linux.zh) | [العربية](https://github.com/Iman/caspian/wiki/Install-Linux.ar) | [اردو](https://github.com/Iman/caspian/wiki/Install-Linux.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Install-Linux.tr)

</div>

تحتاج إلى Linux مع systemd 240 أو أحدث وصلاحيات root. المعماريات المقبولة هي <span dir="ltr">`x86_64`</span> و<span dir="ltr">`aarch64`</span> و<span dir="ltr">`armv7l`</span> و<span dir="ltr">`armv6l`</span>. راجع ترتيب واجهتي الشبكة في دليل بدء الاستخدام الإنجليزي.

اقرأ السكربت أولًا. هذا الأمر يعرضه ولا يثبّت البرنامج:

<div dir="ltr" align="left">

```bash
curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh | less
```

</div>

بعد مراجعته، شغّل التثبيت:

<div dir="ltr" align="left">

```bash
sudo /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"
```

</div>

يختار المثبت الملف المناسب ويتحقق من مجموع التحقق المنشور. يتوقف إذا كان النظام غير مدعوم أو لم يتطابق المجموع. لإجراء تحديث، شغّل أمر التثبيت نفسه مجددًا؛ تبقى الإعدادات المحفوظة. يتضمن الدليل الإنجليزي خطوات التثبيت اليدوي والبناء من المصدر.

[التفاصيل بالإنجليزية](https://github.com/Iman/caspian/wiki/Installation#linux-and-raspberry-pi)

[بدء الاستخدام (English)](https://github.com/Iman/caspian/wiki/Getting-Started)

[ويكي Caspian](https://github.com/Iman/caspian/wiki/Home.ar)

<div dir="ltr" align="left">

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

</div>

</div>
