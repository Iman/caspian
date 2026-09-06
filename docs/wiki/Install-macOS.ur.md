<div dir="rtl" align="right">

# macOS پر تنصیب

<div dir="ltr" align="left">

[English](https://github.com/Iman/caspian/wiki/Install-macOS) | [فارسی](https://github.com/Iman/caspian/wiki/Install-macOS.fa) | [Русский](https://github.com/Iman/caspian/wiki/Install-macOS.ru) | [中文](https://github.com/Iman/caspian/wiki/Install-macOS.zh) | [العربية](https://github.com/Iman/caspian/wiki/Install-macOS.ar) | [اردو](https://github.com/Iman/caspian/wiki/Install-macOS.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Install-macOS.tr)

</div>

macOS 13 یا جدید اور منتظم اکاؤنٹ درکار ہیں۔ اندرونی Wi-Fi کو ہاٹ اسپاٹ بنانے کے لیے Ethernet سے انٹرنیٹ فراہم کریں۔

1. سرکاری ریلیز صفحے سے اپنے پروسیسر کا DMG منتخب کریں: Apple Silicon کے لیے <span dir="ltr">`arm64`</span>، Intel کے لیے <span dir="ltr">`amd64`</span>۔
2. فائل کھولیں اور <span dir="ltr">`Caspian.app`</span> کو Applications میں منتقل کریں۔ اسی فولڈر کی ایپ کھولیں۔
3. اگر Apple کی تصدیق نہ کر سکنے کا انتباہ آئے تو Done منتخب کریں۔ فائل کا ماخذ جانچنے کے بعد System Settings میں Privacy & Security کھولیں اور Caspian کے پاس Open Anyway منتخب کریں۔
4. منتظم کے پاس ورڈ سے تنصیب کی اجازت دیں۔ پہلی تنصیب میں دکھایا گیا پینل پاس ورڈ محفوظ کریں۔
5. Caspian is ready کا انتظار کریں، پھر Open panel کھول کر Wi-Fi اور پراکسی کی ترتیبات درج کریں۔

اگر ایپ کی منظوری کے بعد بھی پس منظر کی فائل <span dir="ltr">`caspian`</span> بند ہو تو اس پر قرنطینہ کی صفت باقی ہو سکتی ہے۔ درج ذیل حکم صرف غیر تصدیق شدہ ڈویلپر یا Apple کی توثیق نہ ہونے کے انتباہ کے لیے ہے، وہ بھی ماخذ اور چیک سم جانچنے کے بعد۔

اگر انتباہ Trojan کا نام لے یا نقصان دہ سافٹ ویئر بتائے تو تنصیب روک دیں اور یہ حکم نہ چلائیں۔ انتباہ کا اصل متن، شناخت کا نام، ریلیز نمبر اور ڈاؤن لوڈ لنک رپورٹ کریں۔

<div dir="ltr" align="left">

```bash
sudo xattr -d com.apple.quarantine /usr/local/bin/caspian
```

</div>

پھر Advanced options میں Restart services منتخب کریں۔ یہ حکم صرف اسی فائل کی قرنطینہ صفت ہٹاتا ہے؛ نہ اسکین کرتا ہے، نہ دستخط کرتا ہے۔ <span dir="ltr">`No such xattr`</span> کا مطلب ہے کہ صفت موجود نہیں۔ خرابی جاری رہے تو دوسرے حفاظتی ضابطے ہٹانے کے بجائے رپورٹ کریں۔

Mac، پینل اور Wi-Fi کے پاس ورڈ الگ ہیں۔ پینل کا پاس ورڈ بھول جائیں تو Caspian Control میں Reset password استعمال کریں؛ منتظم کی اجازت ضروری ہے۔

[انگریزی میں تفصیل](https://github.com/Iman/caspian/wiki/Installation#macos-13-or-later)

[Caspian ویکی](https://github.com/Iman/caspian/wiki/Home.ur)

<div dir="ltr" align="left">

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

</div>

</div>
