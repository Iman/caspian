<!-- wiki-navigation:start -->
<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Third-Party-Credits) · [**فارسی**](https://github.com/Iman/caspian/wiki/Third-Party-Credits.fa) · [Русский](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ru) · [简体中文](https://github.com/Iman/caspian/wiki/Third-Party-Credits.zh) · [العربية](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ar) · [Türkçe](https://github.com/Iman/caspian/wiki/Third-Party-Credits.tr) · [اردو](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ur)

</div>

<div dir="rtl" lang="fa">

[ویکی کاسپین](https://github.com/Iman/caspian/wiki/Home.fa) · [عیب‌یابی](https://github.com/Iman/caspian/wiki/Troubleshooting.fa)

</div>
<!-- wiki-navigation:end -->

<div dir="rtl" align="right">

<a id="third-party-code-and-credits"></a>
# کد و اعتبار شخص ثالث

[NOTICE](https://github.com/Iman/caspian/blob/feature/sni/NOTICE) کتابخانه های مرتبط و فایل های توزیع کاسپین را فهرست می کند.
هر جزء بالادستی مجوز خود را حفظ می کند.
شرایط AGPL کاسپین جایگزین آن اطلاعیه های بالادستی نمی شود.

<a id="sni-spoofing"></a>
## جعل SNI

کد و ایده اصلی از **[patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing)** آمده است.
کاسپین بررسی commit `13b78cf7e073f38d9cadcff542faf4a00b0a6de2`.
الگوریتم ClientHello و دست دادن به `internal/snispoof` اطلاع رسانی می کند.
تغییرات کاسپین یکپارچه سازی Go، اعتبار سنجی، مالکیت اتصال، محدودیت منابع، بازگشت مجدد و آزمایشات را اضافه می کند.

فایل های منبع مشتق شده، یادداشت های GPL-3.0-only را حفظ می کنند.
[مجوز GPL بالادست](https://github.com/Iman/caspian/blob/feature/sni/third_party/sni-spoofing/LICENSE.txt) و [انتساب](https://github.com/Iman/caspian/blob/feature/sni/third_party/sni-spoofing/README.md) کامل در مخزن باقی می مانند.
بخش 13 GPLv3 اجازه ترکیب با کد AGPLv3 را می دهد در حالی که هر قسمت شرایط خاص خود را حفظ می کند.
توزیع کنندگان باید اعلامیه ها را حفظ کنند، تغییرات را علامت گذاری کنند، و منبع مربوطه را تحت مجوزهای مربوطه ارائه دهند.
اعتبار به معنای تایید توسط نویسندگان بالادستی نیست.

Windows x64 از **[WinDivert](https://github.com/basil00/WinDivert/tree/v2.2.2)** توسط Basil (basil00) و مشارکت کنندگان استفاده می کند.
کاسپین LGPL-3.0 را از لایسنس دوگانه خود انتخاب می کند.
نصب کننده شامل درایور اصلاح نشده و DLL، بسته مجوز کامل، منبع، و آرشیو منبع نسخه 2.2.2 است.
[جزئیات توزیع WinDivert](https://github.com/Iman/caspian/blob/feature/sni/third_party/windivert/README.md) را ببینید.
ویندوز ARM64 شامل WinDivert نیست و نمی تواند از این ویژگی SNI استفاده کند.

<a id="ideas-and-acknowledgements"></a>
## ایده ها و قدردانی ها

کاسپین همچنین به نویسندگان و مشارکت‌کنندگان این پروژه‌ها برای ایده‌ها و مقایسه‌های اجرایی که به کار SNI آن کمک می‌کند، اعتبار می‌دهد.
کد و فایل های اجرایی آن ها باندل نیستند.

| پروژه | بازنگری بررسی شد | مجوز و استفاده |
| --- | --- | --- |
| [selfishblackberry177/sni-spoof](https://github.com/selfishblackberry177/sni-spoof) | `a0d68c6627e0898cf13176e322420743f69d894f` | هیچ فایل مجوزی یافت نشد. فقط مقایسه |
| [ValdikSS/GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI) | `f593a276f9ec753889f80208c6a7c5cf455df94a` | Apache-2.0; مرجع استراتژی |
| [bol-van/zapret](https://github.com/bol-van/zapret) | `87e058624c72863db53bdaf7fb6f16576dddb6ab` | MIT; استراتژی و مرجع تشخیصی |
| [Floxu1/UAC-SNI-Spoofer-Android](https://github.com/Floxu1/UAC-SNI-Spoofer-Android) | `c68e350f5f9e9cff308c289db261d7c5d034efa2` | مجوز برنامه سطح بالا یافت نشد. فقط ایده ها |
| [therealaleph/sni-spoofing-rust](https://github.com/therealaleph/sni-spoofing-rust) | `d2956025c31d96f0f0a341af4f1a8eda204857c7` | MIT را اعلام می کند. منشأ الگوی GPL نیاز به توضیح دارد. هیچ کدی کپی نشده |

<a id="other-distributed-components"></a>
## سایر اجزای توزیع شده

[تجزیه کننده پیوند اشتراک گذاری](https://github.com/Iman/caspian/blob/feature/sni/third_party/libxray-share/LICENSE) مجوز MIT خود را حفظ می کند.
نصب‌کننده‌های ویندوز همچنین حاوی باینری‌های رسمی Wintun و کمک‌کننده‌های دات‌نت مستقل هستند.
اعلامیه های آنها تحت [third_party](https://github.com/Iman/caspian/blob/feature/sni/third_party) باقی می ماند و در کنار برنامه نصب می شود.
اتصال Go Wintun یک وابستگی زمان اجرا MIT به ویندوز است.

</div>

<!-- English-source-sha256: ceef83c2b7b0779eb04c1fa1c854f35aaf685978f4d17f439ddaa2fd2d01847c -->
