[English](THIRD-PARTY.md) · [فارسی](THIRD-PARTY.fa.md) · [Русский](../README.ru.md) · [中文](../README.zh.md)

# کد و انتساب شخص ثالث

نسخهٔ انگلیسی مرجع این سند است. آزمون‌ها برابری شناسه‌ها و پیوندها را بررسی می‌کنند.
[NOTICE](../NOTICE) کتابخانه‌های پیوندشده و فایل‌های توزیع Caspian را فهرست می‌کند.
هر جزء بالادستی مجوز خود را حفظ می‌کند؛ شرایط AGPL کاسپین جایگزین آن‌ها نیست.

## جعل SNI

منبع اصلی کد و ایده **[patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing)** است.
Caspian تغییر ثبت‌شدهٔ `13b78cf7e073f38d9cadcff542faf4a00b0a6de2` را بررسی کرده است.
قالب ClientHello و الگوریتم دست‌دهی، مبنای `internal/snispoof` هستند.
تغییرات Caspian شامل یکپارچه‌سازی Go، اعتبارسنجی، مالکیت اتصال، محدودیت منابع، بازگردانی وضعیت و آزمون‌ها است.

فایل‌های مشتق‌شده اعلان GPL-3.0-only را حفظ می‌کنند.
[متن کامل GPL بالادستی](../third_party/sni-spoofing/LICENSE.txt) و [انتساب](../third_party/sni-spoofing/README.md) در مخزن موجود است.
بند ۱۳ GPLv3 ترکیب با کد AGPLv3 را مجاز می‌داند و هر بخش شرایط خود را حفظ می‌کند.
توزیع‌کننده باید اعلان‌ها را حفظ کند، تغییرات را مشخص کند و منبع متناظر را طبق مجوزهای مربوط ارائه دهد.
ذکر نام به معنای تأیید Caspian از سوی نویسندگان بالادستی نیست.

Windows x64 از **[WinDivert](https://github.com/basil00/WinDivert/tree/v2.2.2)** اثر Basil (basil00) و مشارکت‌کنندگان استفاده می‌کند.
Caspian از میان مجوزهای دوگانه، LGPL-3.0 را انتخاب می‌کند.
نصب‌کننده درایور و DLL بدون تغییر، مجموعهٔ کامل مجوزها، انتساب و آرشیو منبع v2.2.2 را دارد.
[جزئیات توزیع WinDivert](../third_party/windivert/README.md) را ببینید.
Windows ARM64 شامل WinDivert نیست و نمی‌تواند از این قابلیت SNI استفاده کند.

## ایده‌ها و قدردانی

Caspian از نویسندگان و مشارکت‌کنندگان پروژه‌های زیر برای ایده‌ها و مقایسه‌های پیاده‌سازی که به توسعهٔ SNI کمک کردند، قدردانی می‌کند.
کد و فایل اجرایی این پروژه‌ها همراه Caspian عرضه نمی‌شود.

| پروژه | نسخهٔ بررسی‌شده | مجوز و کاربرد |
| --- | --- | --- |
| [selfishblackberry177/sni-spoof](https://github.com/selfishblackberry177/sni-spoof) | `a0d68c6627e0898cf13176e322420743f69d894f` | فایل مجوز یافت نشد؛ فقط مقایسه |
| [ValdikSS/GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI) | `f593a276f9ec753889f80208c6a7c5cf455df94a` | Apache-2.0؛ مرجع روش‌ها |
| [bol-van/zapret](https://github.com/bol-van/zapret) | `87e058624c72863db53bdaf7fb6f16576dddb6ab` | MIT؛ مرجع روش‌ها و تشخیص |
| [Floxu1/UAC-SNI-Spoofer-Android](https://github.com/Floxu1/UAC-SNI-Spoofer-Android) | `c68e350f5f9e9cff308c289db261d7c5d034efa2` | مجوز سطح برنامه یافت نشد؛ فقط ایده |
| [therealaleph/sni-spoofing-rust](https://github.com/therealaleph/sni-spoofing-rust) | `d2956025c31d96f0f0a341af4f1a8eda204857c7` | MIT اعلام شده، اما منشأ قالب GPL نیاز به روشن‌سازی دارد؛ کدی کپی نشده است |


## دیگر اجزای توزیع‌شده

[تجزیه‌گر پیوند اشتراک](../third_party/libxray-share/LICENSE) مجوز MIT خود را حفظ می‌کند.
نصب‌کننده‌های Windows فایل‌های رسمی Wintun و ابزارهای کمکی مستقل .NET را نیز دارند.
اعلان‌های آن‌ها در [third_party](../third_party) باقی می‌ماند و کنار برنامه نصب می‌شود.
رابط Go برای Wintun در Windows یک وابستگی زمان اجرا با مجوز MIT است.

<!-- Caspian guide navigation -->

راهنماهای Caspian: [راه‌اندازی و پروتکل‌های پشتیبانی‌شده](../README.fa.md) · [جعل SNI برای عبور از DPI: تنظیم و محدودیت‌ها](SNI.fa.md).
