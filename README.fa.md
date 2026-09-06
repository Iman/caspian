# Caspian-BYOC

[English](README.md) | [فارسی](README.fa.md) | [Русский](README.ru.md) | [中文](README.zh.md)

[![ci](https://github.com/Iman/caspian/actions/workflows/ci.yml/badge.svg)](https://github.com/Iman/caspian/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/Iman/caspian?label=release)](https://github.com/Iman/caspian/releases/latest)
[![licence AGPL-3.0-or-later](https://img.shields.io/badge/licence-AGPL--3.0--or--later-blue)](LICENSE)
[![platform Windows, macOS, Raspberry Pi and Linux](https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Raspberry%20Pi%20%7C%20Linux-blue)](https://github.com/Iman/caspian/releases/latest)
[![container](https://img.shields.io/badge/ghcr.io-caspian-blue)](https://github.com/Iman/caspian/pkgs/container/caspian)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/Iman/caspian)

![دستگاه‌های شما به وای‌فای جعبه وصل می‌شوند. جعبه با کانفیگی که پیست کرده‌اید وصل می‌شود و همه چیز را به سرورِ خودتان در خارج تونل می‌کند، پس مودمِ خانه و شرکتِ اینترنتی فقط یک اتصالِ رمزگذاری‌شده به یک آدرس می‌بینند، نه اینکه شما چه باز می‌کنید.](docs/images/flow-fa.svg)

Caspian-BYOC یک کامپیوتر Windows، یک Mac با macOS، یک Raspberry Pi یا یک
دستگاه Linux را به دروازهٔ وای‌فای تبدیل می‌کند که کانفیگش را خودتان می‌آورید.
یک کانفیگ سازگار با V2Ray یا Xray را در پنل وب پیست کنید و یک کلید را بزنید.
Caspian لینک‌های VLESS،
VMess، Shadowsocks، SOCKS، Trojan و Hysteria2 را می‌پذیرد. فایل‌های YAML مربوط
به Clash و Clash.Meta، فایل JSON خامِ Xray، فهرست لینک‌ها و اشتراک base64 نیز
پذیرفته می‌شوند. Caspian با Xray-core وصل می‌شود و تونل را به شکل هات‌اسپات
وای‌فای پخش می‌کند، پس هر دستگاهی که وصل شود بدون نصب برنامه محافظت می‌شود.

![پنل کاسپین، متصل](docs/images/panel-fa.png)

تصویرِ بالا یک اسکرین‌شاتِ واقعی از یک دستگاهِ در حالِ کار است، گرفته‌شده روی یک
Raspberry Pi 5 در تاریخ 2026-09-03 با تونلِ بالا و پیش از آنکه دستگاهی وصل شود.
رمزِ شبکه، نامِ کانفیگ و آدرسِ سرور در آن جایگزین شده‌اند، و کدِ تصویریِ اتصال تار
شده، چون آن کد نامِ شبکه و رمزش را در خودش دارد. هیچ چیزِ دیگری تغییر نکرده است.

پنل اول فارسی است و بعد انگلیسی. هیچ حسابی در کار نیست، هیچ داده‌ای از شما
فرستاده نمی‌شود، و پنل چیزی از اینترنت نمی‌گیرد.

![Caspian Control در Windows](docs/images/caspian-control-windows.png)

## نصب و راهنماها

[دانلودها](https://github.com/Iman/caspian/releases/latest) | [ویکی کاسپین](https://github.com/Iman/caspian/wiki/Home.fa)

| موضوع | English | فارسی | Русский | 中文 |
|---|---|---|---|---|
| شروع کار | [English](https://github.com/Iman/caspian/wiki/Getting-Started) | [فارسی](https://github.com/Iman/caspian/wiki/Getting-Started.fa) | [Русский](https://github.com/Iman/caspian/wiki/Getting-Started.ru) | [中文](https://github.com/Iman/caspian/wiki/Getting-Started.zh) |
| نصب | [English](https://github.com/Iman/caspian/wiki/Installation) | [فارسی](https://github.com/Iman/caspian/wiki/Installation.fa) | [Русский](https://github.com/Iman/caspian/wiki/Installation.ru) | [中文](https://github.com/Iman/caspian/wiki/Installation.zh) |
| نصب در Linux و Raspberry Pi | [English](https://github.com/Iman/caspian/wiki/Install-Linux) | [فارسی](https://github.com/Iman/caspian/wiki/Install-Linux.fa) | [Русский](https://github.com/Iman/caspian/wiki/Install-Linux.ru) | [中文](https://github.com/Iman/caspian/wiki/Install-Linux.zh) |
| نصب در macOS | [English](https://github.com/Iman/caspian/wiki/Install-macOS) | [فارسی](https://github.com/Iman/caspian/wiki/Install-macOS.fa) | [Русский](https://github.com/Iman/caspian/wiki/Install-macOS.ru) | [中文](https://github.com/Iman/caspian/wiki/Install-macOS.zh) |
| نصب در Windows | [English](https://github.com/Iman/caspian/wiki/Install-Windows) | [فارسی](https://github.com/Iman/caspian/wiki/Install-Windows.fa) | [Русский](https://github.com/Iman/caspian/wiki/Install-Windows.ru) | [中文](https://github.com/Iman/caspian/wiki/Install-Windows.zh) |
| پروتکل‌ها و ترابری‌ها | [English](https://github.com/Iman/caspian/wiki/Protocols-and-Transports) | [فارسی](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa) | [Русский](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru) | [中文](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh) |
| معماری و جریان داده | [English](https://github.com/Iman/caspian/wiki/Architecture) | [فارسی](https://github.com/Iman/caspian/wiki/Architecture.fa) | [Русский](https://github.com/Iman/caspian/wiki/Architecture.ru) | [中文](https://github.com/Iman/caspian/wiki/Architecture.zh) |
| پنل و تنظیمات | [English](https://github.com/Iman/caspian/wiki/Panel-and-Configuration) | [فارسی](https://github.com/Iman/caspian/wiki/Panel-and-Configuration.fa) | [Русский](https://github.com/Iman/caspian/wiki/Panel-and-Configuration.ru) | [中文](https://github.com/Iman/caspian/wiki/Panel-and-Configuration.zh) |
| امنیت و حریم خصوصی | [English](https://github.com/Iman/caspian/wiki/Security-and-Privacy) | [فارسی](https://github.com/Iman/caspian/wiki/Security-and-Privacy.fa) | [Русский](https://github.com/Iman/caspian/wiki/Security-and-Privacy.ru) | [中文](https://github.com/Iman/caspian/wiki/Security-and-Privacy.zh) |
| توسعه و آزمون | [English](https://github.com/Iman/caspian/wiki/Development-and-Testing) | [فارسی](https://github.com/Iman/caspian/wiki/Development-and-Testing.fa) | [Русский](https://github.com/Iman/caspian/wiki/Development-and-Testing.ru) | [中文](https://github.com/Iman/caspian/wiki/Development-and-Testing.zh) |
| عیب‌یابی و نقص‌های شناخته‌شده | [English](https://github.com/Iman/caspian/wiki/Troubleshooting) | [فارسی](https://github.com/Iman/caspian/wiki/Troubleshooting.fa) | [Русский](https://github.com/Iman/caspian/wiki/Troubleshooting.ru) | [中文](https://github.com/Iman/caspian/wiki/Troubleshooting.zh) |
| انتشار و نگهداری | [English](https://github.com/Iman/caspian/wiki/Releases-and-Maintenance) | [فارسی](https://github.com/Iman/caspian/wiki/Releases-and-Maintenance.fa) | [Русский](https://github.com/Iman/caspian/wiki/Releases-and-Maintenance.ru) | [中文](https://github.com/Iman/caspian/wiki/Releases-and-Maintenance.zh) |
| پروانه و منابع پروژه | [English](https://github.com/Iman/caspian/wiki/Licence-and-Credits) | [فارسی](https://github.com/Iman/caspian/wiki/Licence-and-Credits.fa) | [Русский](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ru) | [中文](https://github.com/Iman/caspian/wiki/Licence-and-Credits.zh) |
| نقشهٔ مستندات | [English](https://github.com/Iman/caspian/wiki/Documentation-Map) | [فارسی](https://github.com/Iman/caspian/wiki/Documentation-Map.fa) | [Русский](https://github.com/Iman/caspian/wiki/Documentation-Map.ru) | [中文](https://github.com/Iman/caspian/wiki/Documentation-Map.zh) |
| ترجمه‌ها | [English](https://github.com/Iman/caspian/wiki/Translations) | [فارسی](https://github.com/Iman/caspian/wiki/Translations.fa) | [Русский](https://github.com/Iman/caspian/wiki/Translations.ru) | [中文](https://github.com/Iman/caspian/wiki/Translations.zh) |
| الگوی صفحه | [English](https://github.com/Iman/caspian/wiki/Page-Template) | [فارسی](https://github.com/Iman/caspian/wiki/Page-Template.fa) | [Русский](https://github.com/Iman/caspian/wiki/Page-Template.ru) | [中文](https://github.com/Iman/caspian/wiki/Page-Template.zh) |

## آزمایش‌های ثبت‌شده

> این راهنما از README موجود منتقل شده است. تاریخ اندازه‌گیری‌ها همان تاریخ اصلی است؛ این جابه‌جایی گزارش اجرای تازهٔ آزمون‌ها نیست.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

### چه چیزی از دلِ یک سرورِ واقعی بایت حمل کرده است

`test/tunnel` این را اضافه کرد. هر اسکیمی که تجزیه‌کننده می‌پذیرد، سرتاسر در
برابر یک نمونهٔ واقعیِ xray-core رانده می‌شود که از وابستگیِ خودِ همین ماژول
ساخته شده و با همان بارگذارنده‌ای بار می‌شود که `internal/engine` استفاده
می‌کند. سمتِ کلاینت همان مسیرِ محصول است، بدون تغییر: `link.Parse`، بعد
`xcfg.Build`، بعد `engine.Engine.Start`. هیچ کانفیگی دستی نوشته نمی‌شود.

| پروتکل | ترابری | امنیت | یک درخواست HTTP را حمل می‌کند |
|---|---|---|---|
| VLESS | tcp (raw) | none | بله |
| VMess | tcp (raw) | none | بله |
| Shadowsocks، aes-256-gcm | tcp (raw) | none | بله |
| SOCKS | tcp (raw) | none | بله |
| Trojan | tcp (raw) | TLS، سنجاق‌شده با digest | بله |
| Hysteria2، و نام مستعارِ `hy2` | QUIC | TLS، سنجاق‌شده با digest | بله |

چهار کنترل جلوی قبول شدنِ درخواستی را که از تونل رد نشده می‌گیرند، و هر چهار تا
اجرا می‌شوند نه اینکه در متن ادعا شوند. به کلاینت هرگز گفته نمی‌شود مبدأ کجاست؛
به او یک نامِ `.invalid` و پورتِ یک طعمه داده می‌شود. آن نام قابل ترجمه نیست، و
اگر resolver ای روی دستگاه با این حال جوابش را بدهد، مجموعهٔ آزمون بلند
می‌گویدش. مبدأ بررسی می‌کند درخواست به کجا خطاب شده بود، نه فقط اینکه رسیده است.
طعمه ضربه‌های خودش را می‌شمارد، و یک درخواستِ تونل‌شده نباید حتی یکی به آن اضافه
کند. `TestEveryCarriageProofCanFail` و
`TestTheProofRejectsARequestThatDidNotGoThroughTheTunnel` همان‌هایی هستند که این
کنترل‌ها را از قصد به شاهد تبدیل می‌کنند.

هر سطر را تنگ بخوانید. هر سطر جز Hysteria2 روی TCP خام اجرا می‌شود. هیچ سطری
REALITY را نمی‌راند، چون سمتِ سرورِ آن به یک هدفِ دست‌دادنِ واقعی نیاز دارد.
Shadowsocks فقط aes-256-gcm است، چون رمزهای 2022 مسیرِ کدِ دیگری دارند. هر سطر
یک درخواست TCP حمل می‌کند و UDP associate خاموش است. همه چیز روی loopback است،
پس هیچ آدرسِ خروجی‌ای گرفته نمی‌شود و نمی‌تواند گرفته شود.

`TestEveryProtocolTheParserAcceptsIsDrivenEndToEnd` فهرستِ اسکیم‌های
پذیرفته‌شده را از کدِ `internal/link` می‌خواند، پس اسکیمِ هشتم بدون یک سطر در
اینجا اضافه نمی‌شود.



### واقعاً چه چیزی روی سخت‌افزار اثبات شده است

جدول زیر چیزی است که ترافیکِ واقعی از آن عبور کرده و آدرسِ خروجی‌اش گرفته شده.
این نه آن چیزی است که تجزیه‌کننده می‌پذیرد، و نه آن چیزی که مجموعهٔ آزمونِ
loopback حمل می‌کند.

| پروتکل | ترابری | امنیت | سرتاسر اثبات شده |
|---|---|---|---|
| VLESS | tcp (raw) | REALITY | بله، روی سه سرور جداگانه |
| VLESS | ws (WebSocket) | none، به‌علاوهٔ VLESS Encryption | بله |
| VLESS | ws (WebSocket) | TLS | بله، از راه یک CDN |
| VLESS | httpupgrade | TLS | بله، از راه یک CDN |
| VLESS | xhttp | TLS | بله |
| VMess، Trojan، Shadowsocks، SOCKS، Hysteria2 | هر کدام | هر کدام | نه |

هر کدام از این‌ها با راندنِ یک مرورگر واقعی روی یک گوشیِ واقعیِ وصل به
هات‌اسپات اثبات شده است. آدرسِ خروجی از دو منبعِ مستقل گرفته و با سروری که
پیکربندی نام می‌برد تطبیق داده شد. سه سرور متفاوت به کار رفت و هر کدام آدرس
متفاوتی برگرداند، پس خواندنی که تکراری یا از حافظهٔ نهان باشد را نمی‌شود با
تونلِ سالم اشتباه گرفت.

سطری که اثبات نشده، ادعای خراب بودن نیست. ادعای این است که هیچ‌کس ندیده بسته‌ای
از سرِ دیگر بیرون بیاید، که چیزِ دیگری است و تنها چیزی است که این پروژه شاهد
می‌داند. سندِ نرم‌افزار اتصالی که هر ترابری تولید می‌کند **به‌عنوان یک فایل
golden سنجاق شده است**، پس تغییر در نحوهٔ ساخته شدنش به شکل یک diff دیده
می‌شود. این ثابت می‌کند سند پایدار است و دربارهٔ اینکه ترابری وصل می‌شود یا نه
چیزی نمی‌گوید.



## واقعاً چه چیزی وارسی شده است

### مجموعهٔ آزمونِ Go، همین مخزن، اندازه‌گیری‌شده در 2026-08-31

روی کامیتِ `5b0a8a7` با درختِ کاریِ تمیز، روی go1.27.0 darwin/arm64:

    go build ./...                 exit 0
    go test -count=1 -v ./...      exit 0

آن اجرا 1577 آزمون شاملِ زیرآزمون‌ها را اجرا کرد: 1572 قبول، 5 رد شده (skip)، 0
مردود. پانزده بسته `ok` گزارش دادند. دو بسته هیچ فایلِ آزمونی ندارند:
`bdd/harness` و `local/devpanel`. آن 5 رد شدن اعلام می‌کنند چه چیزی را اثبات
نمی‌کنند: چرخهٔ عمرِ دستگاهِ TUN، که فقط لینوکسی است و به root و `/dev/net/tun`
نیاز دارد؛ سه بررسیِ پیکربندیِ dnsmasq که به نصب بودنِ dnsmasq نیاز دارند؛ و یک
تخلیهٔ PNG از QR که باید خودتان روشنش کنید.

اجرای ثبت‌شدهٔ قبلی، روی کامیتِ `dd15ad6` در 2026-08-30، تعداد 1323 آزمون شاملِ
زیرآزمون‌ها را اجرا کرد: 1319 قبول، 4 رد شده، 0 مردود، در دوازده بسته که `ok`
گزارش دادند.

**`-count=1` اختیاری نیست.** حافظهٔ نهانِ نتیجه را از کار می‌اندازد. بدون آن،
اجرای دوم خط‌های PASS اجرای اول را چاپ می‌کند و با کد 0 بیرون می‌آید، در حالی که
هیچ چیز اجرا نکرده است.

دروازهٔ کامل [`scripts/gate.sh`](https://github.com/Iman/caspian/blob/main/scripts/gate.sh) است: gofmt، `go vet`، کلِ مجموعهٔ آزمون همراه با
race detector، و کفِ پوشش برای هر بسته. پیش از آنکه آن را به لوله‌ای بدهید
سرآیندش را بخوانید. یک لولهٔ shell وضعیتِ آخرین فرمانش را برمی‌گرداند، و همین تله
پیش‌تر در این پروژه یک سبزِ دروغین ساخته است.

[`packaging/test-install.sh`](https://github.com/Iman/caspian/blob/main/packaging/test-install.sh) آن دو اسکریپتِ shell را روی هر دستگاهی که bash دارد
پوشش می‌دهد، از جمله دستگاهی که نصب روی آن ممکن نیست.

### مجموعهٔ آزمونِ رفتار

[`docs/BEHAVIOUR.md`](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md) تعداد 24 سناریو را فهرست می‌کند. اجرای 2026-08-31 هر 24 تا را
اجرا کرد، و `TestEveryScenarioCanFail` تعداد 24 نقصِ تزریق‌شدهٔ متناظر را اجرا
کرد، پس هر سناریو دیده شده که برای همان چیزِ مشخصی که ادعای تشخیصش را دارد قرمز
شود. `TestBehaviourDocumentListsEveryScenario` اگر سند و مجموعهٔ آزمون از هم
فاصله بگیرند، در هر دو جهت، شکست می‌خورد. برای اجرای یکی:

    go test ./test/bdd/ -run 'TestBehaviour/the_firewall'

### مجموعهٔ آزمونِ حمل

`test/tunnel` هر کدام از هفت اسکیمی را که تجزیه‌کننده می‌پذیرد از دلِ یک سرورِ
واقعیِ xray-core روی loopback می‌راند. هر کدام باید یک درخواستِ HTTP را به مبدأیی
برساند که فقط سمتِ دورِ تونل به آن دسترسی دارد. برای اینکه هر سطر چه چیزی را
پوشش می‌دهد و چه چیزی را نه، بخشِ «چه چیزی از دلِ یک سرورِ واقعی بایت حمل کرده
است» در بالا را ببینید. پیش از آنکه این بسته وجود داشته باشد، قوی‌ترین جمله‌ای که
دربارهٔ شش تا از آن هفت پروتکل برقرار بود این بود که نرم‌افزار اتصال سندی را که
تولید می‌کنند بار می‌کند.

### روی سخت‌افزارِ هدف

[`internal/netcfg/testdata/PROVENANCE.md`](https://github.com/Iman/caspian/blob/main/internal/netcfg/testdata/PROVENANCE.md) همان ثبت است، و دربارهٔ تفاوتِ آنچه
گرفته شده و آنچه نوشته شده دقیق است. نامِ فایل‌ها کلاس را حمل می‌کنند:
`capture-pi5-` خروجیِ بایتیِ یک فرمانِ واقعی روی Pi است، `scenario-` دستگاهی است
که هیچ‌کس اندازه‌اش نگرفته، و `golden-` خروجیِ خودِ این پروژه است.

آنچه روی Pi اندازه‌گیری و همان‌جا ثبت شد: هر پنج مجموعه‌قواعدِ تولیدشدهٔ متمایز
که `nft -c -f` آن‌ها را تجزیه کرد، با sha256 هر فایل که روی خودِ Pi خوانده شد و
نه روی دستگاهِ توسعه‌دهنده، و `nft list ruleset` که پیش و پس از آن خالی بود.
توالیِ آزاد کردنِ رابط و وارونه‌هایش. درایوری که از ساختنِ رابطِ AP دوم سر باز
می‌زند در حالی که تغییرِ نوعِ رابطِ موجود را می‌پذیرد. تحریک‌های کلیدِ قطع همراه
با کنترل‌های منفی‌شان. و قفل‌شدنی که سیاستِ input باعثش شد و همان سیاست را
پس گرفت.

آن فایل همچنین ثبت کرده که جایگزین کردنِ بایت‌های نوشته‌شده با بایت‌های
اندازه‌گیری‌شده چه چیزهایی را شکست، که همان استدلالِ نگه داشتنِ هر دو نوع است.
چند نقص در هر اجرای پیشین سبز مانده بودند، از جمله یک مجموعه‌قواعدِ فایروال که
هیچ هسته‌ای بارش نمی‌کرد، و یک برچیدن که روی دستگاهی که فیلترِ مسیرِ معکوس روشن
داشت آن را خاموش می‌کرد.

### سرتاسر، با یک گوشیِ واقعی

بستر [`test/hardware/caspian-hw`](https://github.com/Iman/caspian/blob/main/test/hardware/caspian-hw) است و دستورکارش [`docs/HARDWARE-TEST.md`](https://github.com/Iman/caspian/blob/main/docs/HARDWARE-TEST.md).
استانداردش استانداردِ خودِ این پروژه است. یک اتصال نتیجه نیست، و یک ترابری فقط
وقتی اثبات شده که ترافیکِ واقعی از آن عبور کرده و آدرسِ خروجی گرفته و با سروری که
کانفیگ نام می‌برد تطبیق داده شده باشد. آدرسِ خروجی‌ای که با مبنای بدونِ تونل یکی
باشد یک نشت است و از هر چیزِ دیگری در آن اجرا بالاتر می‌ایستد. گوشی‌ای که وسطِ
ضبط وضعیتِ شبکه‌اش را عوض کند، خواندن را VOID می‌کند، نه قبول و نه نشت.

یک اجرای ثبت‌شده در 2026-08-30، با نامِ `run-20260830T144015Z`، روی IPv4 این نمره
را داد:

- دو کانفیگ اثبات شدند، هر کدام با `verdict PASS` و `sources agree` و یک آدرسِ
  خروجی از هر دو منبعِ مستقل، تطبیق‌داده‌شده با دستگاهی که کانفیگ نامش را می‌برد.
- اثرِ انگشتِ خروجی وقتی کانفیگ عوض شد تغییر کرد.
- بررسیِ DNS در یک پنجرهٔ 30 ثانیه‌ای، یک برچسبِ `.invalid` تصادفیِ مخصوصِ همان
  اجرا را صفر بار به‌صورتِ آشکار روی رابطِ اینترنت پیدا کرد. چهار بستهٔ DNS آشکار
  در همان پنجره از آن رابط عبور کردند، و آن‌ها مالِ خودِ دستگاه‌اند، که طراحی
  بیرونِ تضمین می‌گذاردشان. دقیقاً به همین دلیل است که این بررسی دنبالِ برچسبی
  می‌گردد که هیچ چیزِ دیگری روی شبکه نمی‌توانست تولیدش کند، به‌جای شمردنِ
  بسته‌های پورت 53، که نمی‌توانند پرسشِ فرارکردهٔ یک دستگاه را از پرسشِ خودِ
  دستگاهِ کاسپین تشخیص دهند.
- fail-closed: با متوقف بودنِ نرم‌افزار اتصال و برداشته شدنِ اینترنتِ همراهِ گوشی
  با حالت پرواز، هیچ‌کدام از دو منبع به اینترنت نرسید، در حالی که پنل هنوز از
  روی هات‌اسپات جواب می‌داد. پس این فایروال بود که ترافیک را رد می‌کرد، نه یک
  لینکِ مرده. دو تلاشِ قبلی در همان گام نمرهٔ VOID گرفتند و به‌جای گزارش شدن،
  دوباره گرفته شدند.

دو نکته دربارهٔ آن ثبت. در تلاشِ سوم روی آخرین گام به قبولی رسید، و آن دو خواندنِ
VOID در دفتر هستند و پاک نشده‌اند. و فراورده‌های آن اجرا زیرِ `local/` زندگی
می‌کنند که در gitignore است، پس **در این مخزن نیستند**. اگر این را کلون کنید،
نمی‌توانید آن اجرا را وارسی کنید. فقط می‌توانید خودتان بستر را دوباره اجرا کنید.

دو منبع به کار می‌رود چون یکی می‌تواند از حافظهٔ نهان یا کهنه باشد، و هر دو به
آدرس‌های IP سنجاق شده‌اند نه به نام‌ها. [`docs/HARDWARE-TEST.md`](https://github.com/Iman/caspian/blob/main/docs/HARDWARE-TEST.md) دلیلش را در
پاراگرافی توضیح می‌دهد که خودش آن را مهم‌ترین پاراگرافِ آن فایل می‌نامد. resolver
آن شبکهٔ محلی سرویس‌های اعلامِ IP را به چاهک می‌فرستد. پس دستگاهی که هیچ چیز جز
سرورِ DNS را عوض نکرده باشد و اصلاً هیچ ترافیکی را تونل نکرده باشد، دقیقاً همان
امضایی را نشان می‌داد که بستری با ترجمهٔ نام دنبالش می‌گردد. و نمرهٔ قبول
می‌گرفت.

بستر هر کانفیگ، آدرسِ سرور، شناسهٔ کاربر و کلید را از هر چه می‌نویسد پاک می‌کند.
هر فراورده را دوباره می‌خواند تا مطمئن شود پاک‌سازی گرفته، و یک جاروی جداگانه
دارد که کلِ اجرا را برای هر چیزی که از فیلتر فرار کرده دوباره می‌خواند. هیچ ضبطِ
بسته‌ای هرگز از Pi بیرون نمی‌رود. خروجیِ tcpdump روی خودِ دستگاه به دو عدد
فروکاسته می‌شود، چون یک ضبط روی آن رابطِ اینترنت، ضبطِ مرورِ خودِ نگهدارندهٔ
پروژه است.

<details>
<summary>نمودارهای معماری و شبکه</summary>

```mermaid
flowchart LR
    subgraph device["دستگاهی که به هات‌اسپات وصل شده"]
        BR["مرورگر<br/>پورت 8088 روی آدرس هات‌اسپات"]
    end

    subgraph panelproc["caspian serve --panel، با حساب caspian اجرا می‌شود"]
        PANEL["internal/panel<br/>مسیرها، نشست‌ها، متن‌ها، رندر"]
        STATE["internal/state<br/>تنها نویسندهٔ state.json"]
        LINK1["internal/link<br/>تجزیهٔ لینکِ پیست‌شده"]
        ENG1["internal/engine<br/>فقط Validate، هیچ سوکتی باز نمی‌کند"]
    end

    subgraph privproc["caspian serve --privileged، با root اجرا می‌شود"]
        SVC["internal/privsvc<br/>Service.Start, Stop, Cut, Restore, Recover"]
        XCFG["internal/xcfg<br/>ساختنِ سندِ نرم‌افزار اتصال"]
        NETCFG["internal/netcfg<br/>مسیرها، nftables، دفترچهٔ برچیدن"]
        HOT["internal/hotspot<br/>hostapd و dnsmasq"]
        ENG2["internal/engine<br/>xray-core، در همین فرایند"]
    end

    BR --> PANEL
    PANEL --> STATE
    PANEL --> LINK1
    PANEL --> ENG1
    PANEL -->|"/run/caspian/priv.sock<br/>0660 root:caspian"| SVC
    SVC --> XCFG
    SVC --> NETCFG
    SVC --> HOT
    SVC --> ENG2
```

```mermaid
flowchart TB
    LINK["internal/link<br/>لینک اشتراک‌گذاری می‌آید، یک outbound بیرون می‌رود.<br/>در هیچ فیلد صادرشده‌ای کلیدی حمل نمی‌کند"]
    XCFG["internal/xcfg<br/>هر چه دورِ outbound است:<br/>ورودی TUN، SOCKS، DNS محلی، مسیریابی"]
    ENGINE["internal/engine<br/>xray-core را شروع و متوقف می‌کند.<br/>هر خط را در ورود پاک‌سازی می‌کند"]
    NETCFG["internal/netcfg<br/>دستگاه را نقشه می‌کشد، مجموعه‌قواعد را می‌سازد،<br/>وارونهٔ هر تغییر را در دفترچه می‌نویسد"]
    HOTSPOT["internal/hotspot<br/>hostapd و dnsmasq را می‌سازد و سرپرستی می‌کند.<br/>هیچ رابطی را تشخیص نمی‌دهد، از رادیو چیزی نمی‌پرسد"]
    STATE["internal/state<br/>state.json، اتمی، 0600"]
    PANEL["internal/panel<br/>رابط وب و واژگانِ خطا"]
    PRIVSVC["internal/privsvc<br/>ترتیبِ گام‌ها، و بازخوانی‌ها"]

    PANEL --> LINK
    PANEL --> STATE
    PRIVSVC --> LINK
    PRIVSVC --> XCFG
    PRIVSVC --> NETCFG
    PRIVSVC --> HOTSPOT
    PRIVSVC --> ENGINE
    LINK --> XCFG
    XCFG --> ENGINE
```

```mermaid
flowchart TB
    subgraph panelowns["فقط caspian serve --panel می‌نویسد"]
        SJ["/var/lib/caspian/state.json<br/>0600 caspian. کانفیگ پیست‌شده<br/>و رمز هات‌اسپات را نگه می‌دارد"]
    end

    subgraph privowns["فقط caspian serve --privileged می‌نویسد"]
        JN["/var/lib/caspian/netcfg.journal<br/>0600 root. وارونهٔ هر تغییر،<br/>نوشته‌شده پیش از خودِ تغییر"]
        HC["/run/caspian/hostapd.conf<br/>0600 root، tmpfs، در هر شروع بازنویسی می‌شود"]
        DC["/run/caspian/dnsmasq.conf<br/>0600 root، tmpfs، در هر شروع بازنویسی می‌شود"]
    end

    subgraph nofile["در حافظه نگه داشته می‌شود و در هیچ فایلی نوشته نمی‌شود"]
        CUT["قطع ترافیک"]
        EVT["فهرست رویدادهای پنل"]
        RING["حلقهٔ گزارشِ نرم‌افزار اتصال"]
    end
```

```mermaid
sequenceDiagram
    autonumber
    participant U as کسی که پای پنل است
    participant PA as internal/panel
    participant LK as internal/link
    participant EN as internal/engine
    participant PS as internal/privsvc، با root
    participant NC as internal/netcfg
    participant HS as internal/hotspot

    U->>PA: POST /power, on=1
    PA->>LK: link.Parse روی متنِ ذخیره‌شده
    Note over LK: حالت 1. تجزیه نشد.<br/>کاربر باید متن را درست کند.
    LK-->>PA: یک Link که در هیچ فیلد صادرشده‌ای کلیدی ندارد
    PA->>LK: Link.XrayConfig
    LK-->>PA: یک outbound با تگ proxy، بدون nullها
    PA->>EN: engine.Validate
    Note over EN: حالت 2. خوانده شد، و همان‌طور که هست<br/>قابل استفاده نیست. هیچ سوکتی باز نمی‌شود.
    PA->>PS: StartRequest روی priv.sock
    PS->>PS: کفِ ساعت، تجزیهٔ دوباره، اعتبارسنجی در برابر همین دستگاه
    PS->>NC: Detect، بعد PlanNetwork
    PS->>PS: xcfg.Build، بعد دوباره engine.Validate
    PS->>NC: اعمال PreEngineSteps. فایروال اول است.
    PS->>NC: AssertHotspotInterfaceReleased
    PS->>EN: Engine.Start. دستگاهِ تونل اینجا پیدا می‌شود.
    PS->>NC: اعمال PostEngineSteps. هر گام به تونل یا listener موتور نیاز دارد.
    PS->>HS: Supervisor.Start، اول hostapd و بعد dnsmasq
    PS->>NC: AssertHotspotIsAccessPoint
    PS->>PS: آزمودنِ سرور
    Note over PS: حالت 3. لینک سالم بود و سرور جواب نداد.<br/>بازگردانی‌ای در کار نیست: دستگاه کاملاً<br/>پیکربندی‌شده است و جلوی ترافیک را گرفته.
    PS-->>PA: nil، یا یک panel.Fault
```

```mermaid
flowchart TB
    DEV["دستگاهِ وصل‌شده<br/>آدرس از dnsmasq"] --> IF["رابط هات‌اسپات"]
    IF --> PRE["زنجیرهٔ nft با نام prerouting، از نوع nat<br/>DNS روی پورت 53 اینجا بازهدایت می‌شود"]
    PRE --> ROUTE{"تصمیم مسیریابی<br/>ip rule از زیرشبکهٔ هات‌اسپات<br/>lookup table 8410"}
    ROUTE -->|"مسیر تونل حاضر است"| TOTUN["oif دستگاهِ تونل است<br/>مسیر پیش‌فرض در table 8410"]
    ROUTE -->|"مسیر تونل برداشته شده"| TOUP["oif رابط اینترنت است"]
    TOTUN --> FW1["زنجیرهٔ nft با نام forward، سیاست drop"]
    TOUP --> FW2["زنجیرهٔ nft با نام forward، سیاست drop"]
    FW1 -->|"iifname hotspot oifname tunnel<br/>ip saddr زیرشبکهٔ هات‌اسپات، accept"| POST["زنجیرهٔ nft با نام postrouting<br/>عمداً خالی، بدون masquerade"]
    FW2 -->|"iifname hotspot oifname uplink، drop<br/>قاعدهٔ مسدودکنندهٔ نشت، اولین قاعدهٔ زنجیره"| DROP["دور ریخته شد"]
    POST --> TUN["دستگاهِ تونل<br/>یک netstack در فضای کاربر، داخل نرم‌افزار اتصال"]
    TUN --> OB["outbound با تگ proxy"]
    OB --> UP["رابط اینترنت<br/>یک مسیرِ میزبانِ سنجاق‌شده به سرور"]
    UP --> SRV["سرور شما"]
```

```mermaid
flowchart TB
    GONE["تونل دیگر ترافیک حمل نمی‌کند"] --> Q{"آیا دستگاهِ تونل هنوز وجود دارد؟"}
    Q -->|"دستگاه حذف شده"| WD["هسته هر مسیری را که از آن می‌گذشت برمی‌دارد"]
    WD --> FB["ترافیک دستگاه‌ها به جدول اصلی برمی‌گردد<br/>و به سمت رابط اینترنت می‌رود"]
    FB --> LB["قاعدهٔ مسدودکنندهٔ نشت منطبق می‌شود:<br/>iifname hotspot oifname uplink، drop"]
    Q -->|"دستگاه هست ولی چیزی به آن سرویس نمی‌دهد"| ENTER["ترافیک وارد دستگاهِ تونل می‌شود"]
    ENTER --> NOWHERE["هیچ چیز آن را نمی‌خواند. جلوتر نمی‌رود."]
    LB --> SAFE["هیچ ترافیکی از دستگاه‌ها بیرون نمی‌رود"]
    NOWHERE --> SAFE
```

```mermaid
flowchart TB
    ASK["دستگاهِ وصل‌شده از هر resolver ای که به آن گفته‌اند،<br/>یا یکی که در خودش کدگذاری شده، روی پورت 53 می‌پرسد"]
    ASK --> RD["nft prerouting روی هات‌اسپات:<br/>udp dport 53 و tcp dport 53 به :53 بازهدایت می‌شوند<br/>آدرس مقصد به همین دستگاه بازنویسی می‌شود"]
    RD --> DM["dnsmasq، بایندشده به رابط هات‌اسپات<br/>/run/caspian/dnsmasq.conf"]
    DM -->|"تنها upstream مجازش یک آدرس loopback است"| LD["شنوندهٔ DNS نرم‌افزار اتصال<br/>127.0.0.1:5354، تگِ ورودی local-dns-in"]
    LD --> R1["قاعدهٔ ruleTagLocalDNS<br/>inboundTag local-dns-in، outbound dns-out"]
    R1 --> APP["اپلیکیشنِ DNS نرم‌افزار اتصال<br/>resolverها از internal/xcfg/resolvers.go"]
    APP --> R2["قاعدهٔ ruleTagResolvers<br/>inboundTag resolver-in، outbound proxy.<br/>بالای قاعدهٔ آدرس‌های خصوصی"]
    R2 --> OB["outbound با تگ proxy"]
    OB --> EXIT["زنجیرهٔ resolver، از سر دیگرِ تونل"]
```

```mermaid
flowchart LR
    DOT["DNS روی TLS<br/>tcp 853"] --> REJ["رد با tcp reset،<br/>تا دستگاه به پورت 53 عقب بنشیند"]
    DOQ["DNS روی QUIC<br/>udp 853"] --> DRP["drop"]
    DOH["DNS روی HTTPS<br/>پورت 443"] --> CAR["مثل هر HTTPS دیگری از تونل حمل می‌شود.<br/>نشت نیست. برای هیچ چیزِ اینجا دیدنی نیست."]
```

```mermaid
flowchart LR
    subgraph modea["حالت A، همان که اندازه‌گیری شده"]
        A1["اترنت<br/>اینترنت را می‌آورد"] --- A2["وای‌فای داخلی<br/>هات‌اسپات می‌شود"]
    end
    subgraph modeb["حالت B، هرگز روی سخت‌افزار واقعی اجرا نشده"]
        B1["وای‌فای داخلی<br/>اینترنت را می‌آورد"] --- B2["آداپتور USB که پشتیبانی از AP را گزارش می‌کند<br/>هات‌اسپات می‌شود"]
    end
```

```mermaid
stateDiagram-v2
    [*] --> Off
    Off --> Running: POST /power on=1
    Running --> Off: POST /power on=0
    Running --> TrafficCut: POST /cut cut=1
    TrafficCut --> Running: POST /cut cut=0
    Running --> Running: POST /recover
    TrafficCut --> Off: دستگاه دوباره راه‌اندازی می‌شود
    Running --> Off: دستگاه دوباره راه‌اندازی می‌شود

    note right of Off
        سرویس ممتاز هنگام راه‌اندازی
        دفترچه را بازپخش می‌کند
        و هیچ چیز را شروع نمی‌کند.
    end note
    note right of TrafficCut
        هات‌اسپات بالاست. دستگاه‌ها وصل‌اند.
        پنل در دسترس است. هیچ چیز عبور داده نمی‌شود.
    end note
```

</details>

## پروانه

AGPL-3.0-or-later. [LICENSE](LICENSE) | [NOTICE](NOTICE) | [English](https://github.com/Iman/caspian/wiki/Licence-and-Credits) | [فارسی](https://github.com/Iman/caspian/wiki/Licence-and-Credits.fa) | [Русский](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ru) | [中文](https://github.com/Iman/caspian/wiki/Licence-and-Credits.zh)
