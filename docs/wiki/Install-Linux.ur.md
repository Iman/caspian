<div dir="rtl" align="right">

# Linux اور Raspberry Pi پر تنصیب

<div dir="ltr" align="left">

[English](https://github.com/Iman/caspian/wiki/Install-Linux) | [فارسی](https://github.com/Iman/caspian/wiki/Install-Linux.fa) | [Русский](https://github.com/Iman/caspian/wiki/Install-Linux.ru) | [中文](https://github.com/Iman/caspian/wiki/Install-Linux.zh) | [العربية](https://github.com/Iman/caspian/wiki/Install-Linux.ar) | [اردو](https://github.com/Iman/caspian/wiki/Install-Linux.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Install-Linux.tr)

</div>

پروسیسر اور ریم: Caspian کے لیے کم از کم ریم، پروسیسر کے کور کی تعداد اور رفتار ابھی پیمائش سے طے نہیں ہوئی۔ وسائل کا استعمال ٹریفک کے حجم، پراکسی پروٹوکول اور بیک وقت رابطوں کی تعداد پر منحصر ہے۔ کم از کم تقاضے شائع کرنے سے پہلے، فارغ حالت اور بوجھ کے دوران وسائل کا استعمال ناپنا ضروری ہے۔

Linux کی ریلیز فائلیں <span dir="ltr">x86-64</span>، <span dir="ltr">ARM64</span> اور <span dir="ltr">ARMv6/ARMv7</span> کے لیے بنائی جاتی ہیں۔ صرف معماری کی مطابقت سے یہ ثابت نہیں ہوتا کہ کارکردگی استعمال کے لیے کافی ہوگی۔

Linux، systemd 240 یا جدید، اور root اختیارات درکار ہیں۔ قبول شدہ ساختیں <span dir="ltr">`x86_64`</span>، <span dir="ltr">`aarch64`</span>، <span dir="ltr">`armv7l`</span> اور <span dir="ltr">`armv6l`</span> ہیں۔ دو نیٹ ورک انٹرفیس کی ترتیب انگریزی آغاز کی رہنمائی میں پڑھیں۔

پہلے اسکرپٹ پڑھیں۔ یہ حکم اسے دکھاتا ہے، نصب نہیں کرتا:

<div dir="ltr" align="left">

```bash
curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh | less
```

</div>

جائزے کے بعد تنصیب چلائیں:

<div dir="ltr" align="left">

```bash
sudo /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"
```

</div>

انسٹالر مناسب فائل منتخب کرتا ہے اور شائع شدہ چیک سم جانچتا ہے۔ غیر معاون نظام یا مختلف چیک سم پر رک جاتا ہے۔ اپ ڈیٹ کے لیے یہی تنصیب کا حکم دوبارہ چلائیں؛ محفوظ ترتیبات برقرار رہتی ہیں۔ دستی تنصیب اور سورس سے بلڈ کی تفصیل انگریزی رہنمائی میں ہے۔

[انگریزی میں تفصیل](https://github.com/Iman/caspian/wiki/Installation#linux-and-raspberry-pi)

[شروع کریں (English)](https://github.com/Iman/caspian/wiki/Getting-Started)

[Caspian ویکی](https://github.com/Iman/caspian/wiki/Home.ur)

<div dir="ltr" align="left">

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

</div>

</div>
