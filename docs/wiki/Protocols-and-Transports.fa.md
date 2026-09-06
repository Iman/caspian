<div dir="rtl" align="right">

# پروتکل‌ها و ترابری‌ها

[English](https://github.com/Iman/caspian/wiki/Protocols-and-Transports) | [فارسی](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa) | [Русский](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru) | [中文](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh)

[ویکی کاسپین](https://github.com/Iman/caspian/wiki/Home.fa)

> این راهنما از README موجود منتقل شده است. تاریخ اندازه‌گیری‌ها همان تاریخ اصلی است؛ این جابه‌جایی گزارش اجرای تازهٔ آزمون‌ها نیست.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## چه چیزی را می‌توانید پیست کنید و چه چیزی رد می‌شود

کانفیگ را خودتان می‌آورید. این چیزی است که جعبه می‌پذیرد، برداشته‌شده از همان کدی
که کارِ پذیرفتن را می‌کند و نه از یک فهرستِ آرزو. هر سطر در برابر <span dir="ltr">`internal/link`</span> و
موتورِ پین‌شده اندازه‌گیری شده است.

| | کار می‌کند | رد می‌شود |
|---|---|---|
| لینک‌های اشتراک | <span dir="ltr">`vless://`</span> <span dir="ltr">`vmess://`</span> <span dir="ltr">`ss://`</span> <span dir="ltr">`socks://`</span> <span dir="ltr">`trojan://`</span> <span dir="ltr">`hysteria2://`</span> <span dir="ltr">`hy2://`</span> | <span dir="ltr">`tuic://`</span> <span dir="ltr">`ssr://`</span> <span dir="ltr">`wireguard://`</span> <span dir="ltr">`anytls://`</span> <span dir="ltr">`naive+https://`</span> <span dir="ltr">`hysteria://`</span> (نسخهٔ ۱) |
| سندهای پیست‌شده | YAML مربوط به Clash و Clash.Meta، xray JSON خام، فهرستی از لینک‌ها هر کدام در یک خط، یک بلوکِ base64 اشتراک | نشانیِ اینترنتیِ اشتراک، سندِ Clash پیچیده‌شده در base64، آرایهٔ JSON، متنی که خطِ اولش کامنت باشد |
| ترابری‌ها | <span dir="ltr">`raw`</span> (که <span dir="ltr">`tcp`</span> هم نوشته می‌شود)، <span dir="ltr">`ws`</span>، <span dir="ltr">`grpc`</span>، <span dir="ltr">`httpupgrade`</span>، <span dir="ltr">`xhttp`</span> (و <span dir="ltr">`splithttp`</span>)، <span dir="ltr">`kcp`</span> و <span dir="ltr">`mkcp`</span> | <span dir="ltr">`h2`</span>، <span dir="ltr">`h3`</span>، <span dir="ltr">`http`</span>، <span dir="ltr">`quic`</span>، <span dir="ltr">`gun`</span> |
| امنیت | <span dir="ltr">`none`</span>، <span dir="ltr">`tls`</span>، <span dir="ltr">`reality`</span> | <span dir="ltr">`xtls`</span> (نوعِ قدیمی)، <span dir="ltr">`allowInsecure`</span> |
| flow در VLESS | <span dir="ltr">`xtls-rprx-vision`</span>، <span dir="ltr">`xtls-rprx-vision-udp443`</span>، یا هیچ | هر مقدارِ دیگری |

<span dir="ltr">`h2`</span> و <span dir="ltr">`h3`</span> در آن ستونِ ردشده نامِ ترابری‌اند. خودِ HTTP/2 و HTTP/3 حمل می‌شوند:
<span dir="ltr">`type=xhttp`</span> با <span dir="ltr">`security=tls`</span>، و ALPN در TLS تعیین می‌کند کدام‌یک. بخشِ
[HTTP/2 و HTTP/3 حمل می‌شوند، با نامی دیگر](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa#http2-و-http3-حمل-میشوند-با-نامی-دیگر)
را ببینید.

شش چیز مردم را غافلگیر می‌کند، پس اینجا آمده‌اند و نه در پانویس:

فقط لینکِ اول به کار می‌رود. چهل سرور را پیست کنید، یکی پیکربندی می‌شود؛ پنل به شما
می‌گوید چند تا پیدا کرده است. <span dir="ltr">`ss://`</span> و <span dir="ltr">`socks://`</span> به شکلِ base64 از اطلاعاتِ کاربر
نیاز دارند و املای سادهٔ <span dir="ltr">`method:password@host`</span> رد می‌شود. REALITY فقط روی <span dir="ltr">`raw`</span> و
<span dir="ltr">`xhttp`</span> و <span dir="ltr">`grpc`</span> کار می‌کند، پس جفت‌کردنش با WebSocket همان لحظهٔ پیست توسط موتور رد
می‌شود و نه بعداً سرِ اتصال. <span dir="ltr">`security=`</span> اینجا باید با حروفِ کوچک نوشته شود، هرچند
خودِ موتور اهمیتی نمی‌دهد، و <span dir="ltr">`TLS`</span> با حروفِ بزرگ به شما <span dir="ltr">`none`</span> گزارش می‌شود. پارامترِ
<span dir="ltr">`plugin=`</span> روی لینکِ <span dir="ltr">`ss://`</span> بدون گفتنِ چیزی نادیده گرفته می‌شود. و نشانیِ اینترنتیِ
اشتراک رد می‌شود چون پنل هیچ چیزی از اینترنت نمی‌گیرد، که یک ویژگیِ عمدی است و نه
قابلیتی که جا مانده باشد.

تصویرِ کامل، از جمله اینکه کدام‌یک از این‌ها بایتِ واقعی حمل کرده‌اند و کدام‌یک روی
سخت‌افزار با ثبتِ آدرسِ خروجی سرتاسری اثبات شده‌اند، در بخشِ
[پروتکل‌ها و ترابری‌ها](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa#پروتکلها-و-ترابریها) آمده است. این‌ها سه ادعای متفاوت‌اند و
این پروژه نمی‌گذارد در هم بروند.

## پروتکل‌ها و ترابری‌ها

یک لینک اشتراک‌گذاری سه چیزِ جدا را حمل می‌کند، و جدا نگه داشتنشان کمک می‌کند:
پروتکلِ پروکسی، ترابری‌ای (transport) که آن را حمل می‌کند، و لایهٔ رمزنگاری‌ای
که دور آن ترابری پیچیده می‌شود. یک لینک VLESS روی WebSocket با TLS و یک لینک
VLESS روی TCP ساده با REALITY، یک پروتکل واحدند که از دو راهِ متفاوت به یک نوع
سرور می‌رسند. آن دو به شکل‌های متفاوتی شکست می‌خورند.

پروتکل‌های پروکسی همان هفت اسکیمی هستند که بالاتر آمد. بیشترِ مثال‌های این سند از
VLESS استفاده می‌کنند، چون REALITY برای همان ساخته شده است. هیچ چیزِ این دستگاه
مخصوص آن نیست. تجزیه‌کننده یک توصیف می‌سازد، <span dir="ltr">`internal/xcfg`</span> دورِ آن یک سندِ
نرم‌افزار اتصال می‌چیند، و باقیِ دستگاه نمی‌داند دارد کدام پروتکل را حمل می‌کند.

ترابری‌ها از xray-core می‌آیند و در تجزیه‌کنندهٔ همراه‌شده نام برده شده‌اند:

- <span dir="ltr">`tcp`</span>، که <span dir="ltr">`raw`</span> هم نوشته می‌شود
- <span dir="ltr">`ws`</span>، برای WebSocket
- <span dir="ltr">`httpupgrade`</span>
- <span dir="ltr">`xhttp`</span>، پروتکلی که پیش‌تر SplitHTTP نام داشت. هر دو املا تجزیه می‌شوند
- <span dir="ltr">`grpc`</span>
- <span dir="ltr">`kcp`</span> و <span dir="ltr">`mkcp`</span>، برای mKCP

<span dir="ltr">`h2`</span> و <span dir="ltr">`http`</span> و <span dir="ltr">`h3`</span> و <span dir="ltr">`quic`</span> در این فهرست نیستند و لینکی که یکی از آن‌ها را
بخواهد رد می‌شود، نه اینکه حمل شود. این‌ها در نسخه‌ای از موتور که اینجا پین شده
حذف شده‌اند، و <span dir="ltr">`TestRemovedTransportsAreRefusedWithASentence`</span> در <span dir="ltr">`internal/link`</span>
همان ردکردن را سرِ جایش نگه می‌دارد.

اینکه این ردکردن چقدر خوب خوانده می‌شود به مسیرِ ورود بستگی دارد. سندِ Clash که
یکی از این‌ها را نام ببرد، جمله‌ای دربارهٔ ترابری می‌گیرد. همان ترابری در پارامترِ
<span dir="ltr">`type=`</span> روی یک لینکِ اشتراک از تجزیه‌کنندهٔ همراه‌شده رد می‌شود و به شکلِ پیام کلیِ
«چیزی در متنِ پیست‌شده لینکِ پروکسی‌ای نبود که این دستگاه بفهمد» بیرون می‌آید، که
درست است و کمکی نمی‌کند. <span dir="ltr">`TestRemovedTransportInAURIIsReportedLessWell`</span> همین
تفاوت را پین می‌کند تا یک شکافِ شناخته‌شده باشد نه یک غافلگیری.

### HTTP/2 و HTTP/3 حمل می‌شوند، با نامی دیگر

اینکه <span dir="ltr">`type=h2`</span> یا <span dir="ltr">`type=quic`</span> رد می‌شود به این معنا نیست که دستگاه نمی‌تواند
آن‌ها را حرف بزند. یعنی املایش جابه‌جا شده است. XHTTP جای هر دو را گرفته و نسخهٔ
HTTP خود را از ALPN در TLS انتخاب می‌کند، نه از نامِ ترابری:

| چه می‌خواهید | چه بنویسید |
|---|---|
| HTTP/3، که همان QUIC است | <span dir="ltr">`type=xhttp`</span> با <span dir="ltr">`security=tls`</span> و <span dir="ltr">`alpn=h3`</span> و <span dir="ltr">`mode=stream-one`</span> |
| HTTP/2 | <span dir="ltr">`type=xhttp`</span> با <span dir="ltr">`security=tls`</span> و هر ALPN‌ای که دقیقاً <span dir="ltr">`h3`</span> نباشد |
| QUIC بدونِ XHTTP | یک لینکِ <span dir="ltr">`hysteria2://`</span> که زیرش QUIC است و به <span dir="ltr">`alpn=h3`</span> نیاز دارد |

این کلیدها دست‌نخورده به موتور می‌رسند: <span dir="ltr">`internal/xcfg`</span> خروجی را به‌شکلِ JSON مبهم
حمل می‌کند و هرگز بازش نمی‌کند، پس <span dir="ltr">`alpn`</span> و <span dir="ltr">`mode`</span> و <span dir="ltr">`xmux`</span> و بلوکِ تنظیمِ QUIC
دقیقاً همان‌طور که پیست شده‌اند می‌رسند.

چهار نکته تعیین می‌کند که h3 بگیرید یا بی‌صدا چیزِ دیگری:

<span dir="ltr">`alpn`</span> باید دقیقاً یک مقدار باشد و آن مقدار باید <span dir="ltr">`h3`</span> باشد. نوشتنِ <span dir="ltr">`alpn=h3,h2`</span>
بدونِ هیچ هشداری به شما HTTP/2 می‌دهد، چون موتور فهرستی با هر طولِ دیگری را
درخواستِ نسخهٔ ۲ می‌گیرد. REALITY هر جا حاضر باشد HTTP/2 را تحمیل می‌کند، پس REALITY
و h3 با هم جمع نمی‌شوند و جفت‌کردنشان به‌جای خطا به شما h2 می‌دهد. <span dir="ltr">`mode`</span> باید
صریح نوشته شود، چون حالتِ پیش‌فرض به <span dir="ltr">`packet-up`</span> می‌رسد و نه به شکلِ <span dir="ltr">`stream-one`</span>
که موتور آن را جانشینِ QUIC نام می‌برد. و <span dir="ltr">`downloadSettings`</span>، برای جداکردنِ مسیرِ
بالا و پایین، همراهِ <span dir="ltr">`mode: stream-one`</span> رد می‌شود؛ آن ترکیب به <span dir="ltr">`stream-up`</span> نیاز
دارد.

یک برخوردِ واژگانی هست که بهتر است صریح گفته شود، چون شبیهِ تناقض خوانده می‌شود:
<span dir="ltr">`type=h3`</span> رد می‌شود و <span dir="ltr">`alpn=h3`</span> لازم است. این‌ها دو فیلدِ متفاوت‌اند. اولی نامِ
ترابری‌ای است که دیگر وجود ندارد؛ دومی نامِ پروتکلی است که داخلِ TLS مذاکره می‌شود.

این پیکربندی‌ها را دستگاه می‌پذیرد و اعتبارسنجی می‌کند. هنوز از اینجا در برابرِ یک
سرورِ واقعی رانده نشده‌اند، پس این سطر را توانِ موتور بدانید و نه چیزی که این پروژه
دیدنش کرده باشد.

لایهٔ امنیت یکی از <span dir="ltr">`reality`</span>، <span dir="ltr">`tls`</span>، یا <span dir="ltr">`none`</span> است.

هر ترکیبی به یک اندازه مفید نیست. REALITY معمولاً با TCP ساده جفت می‌شود، چون کلِ
روشش قرض گرفتنِ دست‌دادنِ TLS یک سایتِ واقعی است، پس پیچیدنش در یک لایهٔ TLS دیگر
هدف را از بین می‌برد. WebSocket و HTTPUpgrade و XHTTP برای این هستند که در چشمِ
چیزی که اتصال را بازرسی می‌کند شبیه ترافیک وبِ معمولی باشند، و معمولاً به همان
دلیلی که یک وب‌سایت معمولی TLS دارد، با TLS جفت می‌شوند. WebSocket با
<span dir="ltr">`security=none`</span> تنها شکلی است که باید دو بار به آن فکر کرد. روی سیم متنِ آشکار
است، و فقط وقتی معقول است که چیز دیگری از پیش رمزنگاری را فراهم کرده باشد، مثل
یک CDN که TLS را جلوی سرور خاتمه می‌دهد.

### سه ادعای متفاوت، جدا از هم

تفکیکِ زیر مهم‌ترین چیز در این سند است. پیش از سطرها، عنوانِ ستون‌ها را بخوانید.

| ادعا | بر چه تکیه دارد | چقدر می‌ارزد |
|---|---|---|
| تجزیه‌کننده آن را می‌پذیرد | <span dir="ltr">`internal/link`</span>، و یک سندِ golden کامیت‌شدهٔ نرم‌افزار اتصال | سند پایدار است. چیزی شماره‌گیری نشده |
| بایت حمل می‌کند | <span dir="ltr">`test/tunnel`</span>، یک سرور واقعیِ xray-core روی loopback | ترافیک از دلِ پروتکل عبور کرد. نه آدرس خروجی، نه دستگاه، نه اینترنت |
| سرتاسر اثبات شده است | <span dir="ltr">`test/hardware`</span>، یک گوشیِ واقعی روی هات‌اسپات | ترافیک واقعی از دستگاه بیرون رفت و آدرسِ خروجی گرفته و نام‌گذاری شد |

### چه چیزی از دلِ یک سرورِ واقعی بایت حمل کرده است

<span dir="ltr">`test/tunnel`</span> این را اضافه کرد. هر اسکیمی که تجزیه‌کننده می‌پذیرد، سرتاسر در
برابر یک نمونهٔ واقعیِ xray-core رانده می‌شود که از وابستگیِ خودِ همین ماژول
ساخته شده و با همان بارگذارنده‌ای بار می‌شود که <span dir="ltr">`internal/engine`</span> استفاده
می‌کند. سمتِ کلاینت همان مسیرِ محصول است، بدون تغییر: <span dir="ltr">`link.Parse`</span>، بعد
<span dir="ltr">`xcfg.Build`</span>، بعد <span dir="ltr">`engine.Engine.Start`</span>. هیچ کانفیگی دستی نوشته نمی‌شود.

| پروتکل | ترابری | امنیت | یک درخواست HTTP را حمل می‌کند |
|---|---|---|---|
| VLESS | tcp (raw) | none | بله |
| VMess | tcp (raw) | none | بله |
| Shadowsocks، aes-256-gcm | tcp (raw) | none | بله |
| SOCKS | tcp (raw) | none | بله |
| Trojan | tcp (raw) | TLS، سنجاق‌شده با digest | بله |
| Hysteria2، و نام مستعارِ <span dir="ltr">`hy2`</span> | QUIC | TLS، سنجاق‌شده با digest | بله |

چهار کنترل جلوی قبول شدنِ درخواستی را که از تونل رد نشده می‌گیرند، و هر چهار تا
اجرا می‌شوند نه اینکه در متن ادعا شوند. به کلاینت هرگز گفته نمی‌شود مبدأ کجاست؛
به او یک نامِ <span dir="ltr">`.invalid`</span> و پورتِ یک طعمه داده می‌شود. آن نام قابل ترجمه نیست، و
اگر resolver ای روی دستگاه با این حال جوابش را بدهد، مجموعهٔ آزمون بلند
می‌گویدش. مبدأ بررسی می‌کند درخواست به کجا خطاب شده بود، نه فقط اینکه رسیده است.
طعمه ضربه‌های خودش را می‌شمارد، و یک درخواستِ تونل‌شده نباید حتی یکی به آن اضافه
کند. <span dir="ltr">`TestEveryCarriageProofCanFail`</span> و
<span dir="ltr">`TestTheProofRejectsARequestThatDidNotGoThroughTheTunnel`</span> همان‌هایی هستند که این
کنترل‌ها را از قصد به شاهد تبدیل می‌کنند.

هر سطر را تنگ بخوانید. هر سطر جز Hysteria2 روی TCP خام اجرا می‌شود. هیچ سطری
REALITY را نمی‌راند، چون سمتِ سرورِ آن به یک هدفِ دست‌دادنِ واقعی نیاز دارد.
Shadowsocks فقط aes-256-gcm است، چون رمزهای 2022 مسیرِ کدِ دیگری دارند. هر سطر
یک درخواست TCP حمل می‌کند و UDP associate خاموش است. همه چیز روی loopback است،
پس هیچ آدرسِ خروجی‌ای گرفته نمی‌شود و نمی‌تواند گرفته شود.

<span dir="ltr">`TestEveryProtocolTheParserAcceptsIsDrivenEndToEnd`</span> فهرستِ اسکیم‌های
پذیرفته‌شده را از کدِ <span dir="ltr">`internal/link`</span> می‌خواند، پس اسکیمِ هشتم بدون یک سطر در
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

### چرا سطری که ترابری‌اش امنیت ندارد باز هم رمزنگاری‌شده است

ستون <span dir="ltr">`security`</span> در بالا دربارهٔ لایه‌ای است که **دورِ** ترابری پیچیده می‌شود، و
<span dir="ltr">`none`</span> در آنجا به معنی «بدون رمزنگاری» نیست. یعنی نه TLS و نه REALITY. دقت در
این مورد می‌ارزد، چون خواندنش به شکل دیگر نگران‌کننده است و خواندنش با سخاوتِ
بیش از حد بدتر.

VLESS به‌تنهایی هیچ رمزنگاری‌ای حمل نمی‌کند. پروتکلی بی‌حالت است که انتظار دارد
لایهٔ زیرینش محرمانگی را فراهم کند، که معمولاً REALITY یا TLS است. یک لینک VLESS
روی WebSocket با <span dir="ltr">`security=none`</span> و هیچ چیزِ دیگر، روی سیم متنِ آشکار **می‌بود**،
و آدرسِ خروجی اثبات می‌شد در حالی که هر بسته برای هر چیزی که روی مسیر بود خواندنی
بود.

آنچه آن سطر را امن می‌کند VLESS Encryption است، که در پارامترِ <span dir="ltr">`encryption=`</span>
لینک حمل می‌شود. این یک تبادلِ کلیدِ ترکیبی است، ML-KEM-768 برای مقاومتِ
پساکوانتومی همراه با X25519، که در خودِ لایهٔ VLESS اعمال می‌شود نه زیرِ آن. پس
ترافیک رمزنگاری‌شده است، و چیزی آن را رمز کرده که ساخته شده تا در برابرِ مهاجمی
که امروز ضبط می‌کند و بعداً کامپیوترِ کوانتومی دارد امن بماند. لینکی که هم
<span dir="ltr">`encryption=none`</span> و هم <span dir="ltr">`security=none`</span> دارد، هیچ‌کدام را ندارد، و همین ترکیب است
که باید رد شود.

این **همان** Noise Protocol Framework (noiseprotocol.org) **نیست**. نه در این
دستگاه، نه در تجزیه‌کنندهٔ همراه‌شدهٔ لینک، و نه در نرم‌افزار اتصال، هیچ چیز
Noise را پیاده نمی‌کند. واژهٔ "noise" در پیکربندیِ xray-core برای چیزِ دیگری
می‌آید، یعنی پر کردنِ ترافیک با بایت‌های تصادفی تا شکلش روی سیم عوض شود، که
مبهم‌سازی است و دست‌دادن نیست. آنچه به این سطر محرمانگی می‌دهد VLESS Encryption
است، و نام اهمیت دارد چون آن دو تضمین‌های متفاوتی می‌دهند.

**اندازه‌گیری شده**، نه فرض‌شده، در 2026-08-30. این بسته outbound را فیلد به
فیلد بازنمی‌سازد. آنچه را تجزیه‌کننده تولید کرده دوباره سریال می‌کند، و تنظیماتِ
پروتکل به شکلِ یک بلوکِ مبهم همراهش می‌آیند. به همین دلیل است که آن پارامتر زنده
می‌ماند. و به همین دلیل هم هست که اگر روزی زنده نماند هیچ چیز نمی‌شکند: هیچ
فیلدی گم نمی‌شود، هیچ نوعی عوض نمی‌شود، و هیچ آزمون دیگری متوجه نمی‌شود، در حالی
که تونل ترافیکِ کاربر را آشکار حمل می‌کند و همهٔ بررسی‌ها هنوز سبزند.
<span dir="ltr">`TestVLESSEncryptionSurvivesIntoTheEngineDocument`</span> در <span dir="ltr">`internal/link`</span> همان
نگهبان است، و پیش از آنکه نگه داشته شود، دیده شد که دقیقاً در برابرِ همان تنزلِ
بی‌سروصدا شکست می‌خورد.

### نامِ گواهی‌ای که نخواند، و اصلاحی که سمتِ کلاینت است

یک نتیجه ثبت کردن دارد، چون ایرادی است که این دستگاه به‌درستی حاضر نمی‌شود رویش
سرپوش بگذارد. دو پیکربندی به آدرسِ خودِ سرور اشاره می‌کردند در حالی که نامِ TLS
مربوط به CDN جلوی آن را حمل می‌کردند. نرم‌افزار اتصال گزارش داد:

<div dir="ltr" align="left">

    transport/internet/httpupgrade: failed to dial request ...
      tls: failed to verify certificate: x509: certificate is valid for
      <the apex>, not <the cdn subdomain>


</div>

این واقعاً گواهی‌ای است که با نامِ درخواست‌شده نمی‌خواند، و رد کردنش همان رفتاری
است که می‌خواهید. پذیرفتنش یعنی تونل را هر چیزی که هر گواهی‌ای در دست دارد بتواند
خاتمه بدهد.

علت و اصلاح هر دو سمتِ کلاینت هستند، و هیچ تغییری در سرور لازم نیست. یک لینکِ
اشتراک‌گذاری دو نام حمل می‌کند که مردم فکر می‌کنند باید یکی باشند و نیستند:

- <span dir="ltr">`sni`</span> نامی که TLS گواهی را در برابرِ آن اعتبارسنجی می‌کند
- <span dir="ltr">`host`</span> نامی که سرور درخواست را بر اساس آن مسیریابی می‌کند، یک هدر HTTP

لینک‌های ناموفق نامِ CDN را در **هر دو** حمل می‌کردند. از راه CDN این کار
می‌کند، چون CDN گواهیِ آن نام را دارد. وقتی مستقیم به مبدأ اشاره کند نمی‌تواند،
چون مبدأ فقط گواهیِ دامنهٔ اصلی را دارد. <span dir="ltr">`sni`</span> را روی نامی بگذارید که گواهی
واقعاً حمل می‌کند، و <span dir="ltr">`host`</span> را همان نامی بگذارید که سرور بر اساس آن مسیریابی
می‌کند:

<div dir="ltr" align="left">

    sni=example.com          host=cdn.example.com


</div>

**اندازه‌گیری شده** در 2026-08-30. دو لینکی که با خطای گواهیِ بالا شکست خورده
بودند، بعد از همان یک تغییر هر دو وصل شدند. آدرس‌های خروجی از دو منبعِ مستقل
گرفته و با سرورهای خودشان تطبیق داده شد، و در همان اجرا، بررسیِ نشتِ DNS و بررسیِ
fail-closed هم قبول شدند.

پس اگر ترابری‌ای فقط وقتی مستقیم به مبدأ اشاره می‌کند شکست می‌خورد، پیش از آنکه
به ترابری شک کنید، <span dir="ltr">`sni`</span> را با نام‌های جایگزینِ موضوعِ گواهیِ مبدأ مقایسه کنید.
<span dir="ltr">`openssl s_client -connect <address>:443 -servername <name>`</span> چاپ می‌کند سرور
واقعاً چه ارائه می‌دهد.

### پنل لینکِ پیست‌شده می‌گیرد، نه تصویر

انداختنِ یک تصویر QR در طراحی، بخش 5.2، توصیف شده و **پیاده نشده است**.
<span dir="ltr">`internal/panel/qr`</span> فقط رمزگذار است، و هیچ هندلری در <span dir="ltr">`internal/panel`</span> بارگذاریِ
multipart نمی‌خواند. کدِ QR ای که پنل تولید می‌کند همانی است که گوشی برای وصل شدن
به هات‌اسپات اسکن می‌کند. [<span dir="ltr">`internal/panel/view.go`</span>](https://github.com/Iman/caspian/blob/main/internal/panel/view.go) آن را با <span dir="ltr">`qr.Encode`</span> و
<span dir="ltr">`qr.WiFiJoin`</span> می‌سازد، پس نه کتابخانهٔ تصویری در کار است و نه سرویس راه دور.

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

[English: HTTP/2, HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports#http2-and-http3-are-carried-under-a-different-name) | [English](https://github.com/Iman/caspian/wiki/Protocols-and-Transports#protocols-and-transports) | [فارسی: HTTP/2, HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa#http2-و-http3-حمل-میشوند-با-نامی-دیگر) | [فارسی](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa#پروتکلها-و-ترابریها) | [Русский: HTTP/2, HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru#http2-и-http3-переносятся-просто-под-другим-именем) | [Русский](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru#протоколы-и-транспорты) | [中文: HTTP/2, HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh#http2-与-http3-是被承载的只是换了个名字) | [中文](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh#协议与传输)

</div>
