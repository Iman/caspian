<div dir="rtl" align="right">

# پروانه و منابع پروژه

[English](https://github.com/Iman/caspian/wiki/Licence-and-Credits) | [فارسی](https://github.com/Iman/caspian/wiki/Licence-and-Credits.fa) | [Русский](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ru) | [中文](https://github.com/Iman/caspian/wiki/Licence-and-Credits.zh)

[ویکی کاسپین](https://github.com/Iman/caspian/wiki/Home.fa)

> این راهنما از README موجود منتقل شده است. تاریخ اندازه‌گیری‌ها همان تاریخ اصلی است؛ این جابه‌جایی گزارش اجرای تازهٔ آزمون‌ها نیست.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## پروانه

AGPL-3.0-or-later، با سه شرطِ اضافی زیرِ بخش 7. هر سه از جنسی هستند که بخش 7
اجازه می‌دهد و هیچ‌کدام آنچه را می‌توانید با نرم‌افزار بکنید محدود نمی‌کند: حفظِ
اعلانِ حقِ نشر، حفظِ همین انتساب و یک ارجاعِ دیدنی به پروژهٔ کاسپین در هر رابط
کاربری؛ علامت‌گذاریِ نسخهٔ خودتان به‌عنوانِ تغییریافته اگر تغییرش دادید؛ و
به‌کارنبردنِ نامِ نویسندگان یا نامِ پروژه برای تبلیغات، که شاملِ جمع‌کردنِ کمکِ مالی،
اسپانسر یا گرنت با آن نام‌ها هم می‌شود. متنِ کامل در [<span dir="ltr">`LICENSE`</span>](https://github.com/Iman/caspian/blob/main/LICENSE) است و شرط‌ها در
[<span dir="ltr">`NOTICE`</span>](https://github.com/Iman/caspian/blob/main/NOTICE).

آن شرطِ سوم فقط کاربردِ نام‌ها را محدود می‌کند و نه چیزِ دیگری را. شما همچنان آزادید
نرم‌افزار را زیرِ AGPL اجرا کنید، مطالعه کنید، تغییر دهید و بازتوزیع کنید، برای هر
هدفی از جمله هدفِ تجاری. آنچه نمی‌توانید بکنید جمع‌کردنِ پول به نامِ نویسندگان است.

AGPL به‌جای GPL، چون این برنامه معمولاً به‌شکلِ سرویسی اجرا می‌شود که دیگران به آن
وصل می‌شوند، و بخش 13 شکافی را می‌بندد که GPL ساده باز می‌گذارد. پروانهٔ آزادتری
نیست، چون این باینری کدِ GPL-3.0-or-later را به‌شکلِ ایستا لینک می‌کند:
<span dir="ltr">`github.com/sagernet/sing`</span> و <span dir="ltr">`github.com/sagernet/sing-shadowsocks`</span>، که هر دو از
راهِ xray-core می‌آیند. پس کارِ ترکیب‌شده باید روی شرایطِ خانوادهٔ GPL باشد و MIT یا
Apache-2.0 برای آن در دسترس نیست.

## ساخته‌شده بر پایهٔ

کاسپین مقدار کمی کد است دورِ کارِ دیگران. موتور xray-core است و تجزیه‌کنندهٔ لینکِ
اشتراک مالِ XTLS است. هیچ‌کدام از این پروژه‌ها این یکی را تأیید نمی‌کنند؛ نامشان
اینجاست چون کار مالِ آن‌هاست.

| پروژه | پروانه | اینجا چه می‌کند |
|---|---|---|
| [xray-core](https://github.com/xtls/xray-core) | MPL-2.0 | موتورِ پروکسی، که در همین پروسه لینک می‌شود و نه به‌عنوانِ برنامه‌ای جدا اجرا |
| [libXray](https://github.com/XTLS/libXray) | MIT | تجزیه‌کنندهٔ لینکِ اشتراک، همراه‌شده زیرِ <span dir="ltr">`third_party/libxray-share/`</span> |
| [REALITY](https://github.com/xtls/reality) | MPL-2.0 | ترابریِ استتارِ TLS |
| [uTLS](https://github.com/refraction-networking/utls) | BSD-3-Clause | تقلیدِ اثرِانگشتِ TLS |
| [quic-go](https://github.com/apernet/quic-go) | MIT | پشتهٔ QUIC که Hysteria2 رویش کار می‌کند |
| [gVisor](https://github.com/google/gvisor) | Apache-2.0 | پشتهٔ شبکهٔ فضای‌کاربر که ورودیِ TUN از آن استفاده می‌کند |
| [sing](https://github.com/sagernet/sing) و [sing-shadowsocks](https://github.com/sagernet/sing-shadowsocks) | GPL-3.0-or-later | Shadowsocks 2022، و دلیلِ اینکه این پروژه کپی‌لفت است |
| [netlink](https://github.com/vishvananda/netlink) | Apache-2.0 | رابط‌ها، آدرس‌ها و مسیرها |
| [miekg/dns](https://github.com/miekg/dns) | BSD-3-Clause | کار با پیام‌های DNS |
| [gorilla/websocket](https://github.com/gorilla/websocket) | BSD-2-Clause | ترابریِ WebSocket |
| [CIRCL](https://github.com/cloudflare/circl) | BSD-3-Clause | تبادلِ کلیدِ پساکوانتومی |

همچنین روی خودِ دستگاه به <span dir="ltr">`hostapd`</span> و <span dir="ltr">`dnsmasq`</span> و <span dir="ltr">`nftables`</span> و <span dir="ltr">`iw`</span> و <span dir="ltr">`iproute2`</span>
نیاز دارد. این‌ها به‌شکلِ برنامه‌های جدا اجرا می‌شوند و لینک نمی‌شوند، پس پروانه‌شان
روی پروانهٔ این کار اثری ندارد، ولی بدونِ آن‌ها این دستگاه هیچ است.

<span dir="ltr">`third_party/libxray-share/`</span> با پروانهٔ MIT است، Copyright (c) 2023-2025 XTLS، و
متنِ پروانه‌اش کنارِ خودِ کد نگهداری می‌شود. [<span dir="ltr">`NOTICE`</span>](https://github.com/Iman/caspian/blob/main/NOTICE) ثبتِ کامل را دارد: هر ماژولی که
در باینری است، پروانه‌ای که از فایلِ پروانهٔ خودش خوانده شده، و استدلالِ سازگاری.

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

## اجزای Windows

| پروژه | پروانه | کاربرد |
|---|---|---|
| [Wintun](https://www.wintun.net/) | Wintun Prebuilt Binaries License | درایور تونل امضاشدهٔ <span dir="ltr">`wintun.dll`</span> |
| [.NET runtime and Windows Forms](https://github.com/dotnet/runtime) | MIT | محیط اجرای همراه برنامهٔ کنترل Windows |
| <span dir="ltr">`System.ServiceProcess.ServiceController`</span> | MIT | کنترل سرویس‌ها از <span dir="ltr">`CaspianControl.exe`</span> |

نصب Windows فایل جداگانهٔ <span dir="ltr">`wintun.dll`</span> را دارد. نسخهٔ رسمی و امضاشدهٔ Wintun 0.14.1 بدون تغییر توزیع می‌شود.
پروانه در [<span dir="ltr">`third_party/wintun/PREBUILT-BINARIES-LICENSE.txt`</span>](https://github.com/Iman/caspian/blob/main/third_party/wintun/PREBUILT-BINARIES-LICENSE.txt) است و هنگام نصب در <span dir="ltr">`C:\Program Files\Caspian\WINTUN-LICENSE.txt`</span> کپی می‌شود.

برنامه‌های <span dir="ltr">`caspian-tethering.exe`</span> و <span dir="ltr">`CaspianControl.exe`</span> محیط اجرای .NET را درون فایل اجرایی خود دارند.
پروانه‌ها و اعلان‌های آن در <span dir="ltr">`third_party/dotnet/`</span> قرار دارند. بستهٔ مرجع Windows SDK ورودی ساخت است و همراه Caspian نصب نمی‌شود.

</div>
