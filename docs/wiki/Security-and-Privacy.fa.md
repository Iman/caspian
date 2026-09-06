<div dir="rtl" align="right">

# امنیت و حریم خصوصی

[English](https://github.com/Iman/caspian/wiki/Security-and-Privacy) | [فارسی](https://github.com/Iman/caspian/wiki/Security-and-Privacy.fa) | [Русский](https://github.com/Iman/caspian/wiki/Security-and-Privacy.ru) | [中文](https://github.com/Iman/caspian/wiki/Security-and-Privacy.zh)

[ویکی کاسپین](https://github.com/Iman/caspian/wiki/Home.fa)

> این راهنما از README موجود منتقل شده است. تاریخ اندازه‌گیری‌ها همان تاریخ اصلی است؛ این جابه‌جایی گزارش اجرای تازهٔ آزمون‌ها نیست.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## چه چیزی را تضمین می‌کند

هر عنوان در اینجا با خروجیِ تولیدشدهٔ فایروال در <span dir="ltr">`internal/netcfg/testdata/`</span>، یا
با یک آزمونِ نام‌دار، یا با اندازه‌گیری‌ای که در مخزن ثبت شده پشتیبانی می‌شود.
[<span dir="ltr">`docs/BEHAVIOUR.md`</span>](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md) فهرستِ خواندنیِ قول‌هاست. هر عنوان در آن، نامِ یک سناریو در
<span dir="ltr">`test/bdd/`</span> است، و هر سناریو یک نقصِ تزریق‌شدهٔ متناظر دارد. پس «این آزمون
می‌تواند همان چیزی را که ادعا می‌کند تشخیص دهد» خودش یک نتیجهٔ آزمون است.

### ترافیکِ عبوریِ دستگاه‌ها fail-closed است، و قاعدهٔ مسدودکننده به تونل نیاز ندارد

اصطلاحِ fail-closed یعنی وقتی تونل نباشد، ترافیک متوقف می‌شود و از راهِ دیگری
بیرون نمی‌رود.

سیاستِ زنجیرهٔ forward برابرِ <span dir="ltr">`drop`</span> است. اولین قاعده در آن، قاعدهٔ مسدودکنندهٔ
نشت است، و فقط نامِ هات‌اسپات و رابطِ اینترنت را می‌برد:

<div dir="ltr" align="left">

    iifname "wlan0" oifname "eth0" drop comment "fail-closed: client traffic never leaves by the uplink"


</div>

هر قاعده‌ای که به ترافیکِ دستگاه‌ها اجازه می‌دهد نامِ دستگاهِ تونل را می‌برد، پس
وقتی تونل ناپدید شود آن قواعد دیگر منطبق نمی‌شوند و سیاست همه چیز را drop
می‌کند. خودِ قاعدهٔ مسدودکننده وقتی تونل برود نمی‌تواند از کار بیفتد، چون نامی از
آن نبرده است. هر رابط با نام تطبیق داده می‌شود و هرگز با شماره، پس مجموعه‌قواعد
بدون حضورِ تونل هم بار می‌شود، و دقیقاً همان وقت است که لازم است. زنجیرهٔ
postrouting عمداً خالی است.

سناریو: "with the tunnel gone, nothing lets client traffic out by the uplink".
آزمونِ پشتیبانِ تحلیل‌گر: <span dir="ltr">`TestWithoutInterfaceRemovesOnlyTheRulesNamingIt`</span>.

### کلیدِ قطع، ترافیکِ خودِ دستگاه را هم پوشش می‌دهد

زنجیرهٔ output سیاستِ <span dir="ltr">`drop`</span> دارد با یک فهرستِ اجازهٔ نام‌دار. اجازه‌ها از شمردنِ
آنچه واقعاً روی دستگاهِ هدف اجرا می‌شود به دست آمده‌اند، نه از نمونه‌برداریِ
ترافیک، و هر اجازه در مجموعه‌قواعدِ تولیدشده، خواندنی را که آن را توجیه می‌کند با
خود دارد: سوکت‌های کلاینتِ DHCP از NetworkManager، systemd-timesyncd، DNS،
loopback، دستگاهِ تونل، کشفِ همسایهٔ IPv6، و سرورِ پروکسی که **با آدرس** اجازه
دارد نه با پورت، تا ترابری‌ای که روی UDP و پورت 443 است بی‌سروصدا نشکند. یک
اجازه از استدلال آمده نه از اندازه‌گیری، و خودش این را می‌گوید: جواب دادنِ دستگاه
به DHCP در نقشِ سرور روی هات‌اسپات، که conntrack نمی‌تواند پوششش دهد چون پاسخِ
DHCP و درخواستش هیچ چندتاییِ مشترکی ندارند.

تحریک‌ها و مهم‌تر از آن، کنترل‌های منفی، در <span dir="ltr">`PROVENANCE.md`</span> زیر عنوانِ
"The three provocations, run with the policy loaded" ثبت شده‌اند. آزمون‌ها:
<span dir="ltr">`TestRestrictedEgress_PermitList`</span>،
<span dir="ltr">`TestRestrictedEgress_AcceptsEstablishedBeforeItDropsAnything`</span>،
<span dir="ltr">`TestRestrictedEgress_ServerIsPermittedByAddressNotPort`</span>.

**هزینه در سرآیندِ خودِ مجموعه‌قواعد نوشته شده و لازم نیست کشف شود: تا وقتی
دستگاه روشن است، <span dir="ltr">`apt update`</span> از یک shell روی دستگاه شکست می‌خورد.**

### یک قطعِ اضطراری هست که هات‌اسپات را با خودش نمی‌برد

خاموش کردنِ دستگاه هات‌اسپات را هم پایین می‌آورد، که گوشیِ نگه‌دارندهٔ دکمه را
قطع می‌کند. پس کنترلِ جداگانه‌ای هست که ترافیکِ عبوریِ دستگاه‌ها را قطع می‌کند و
هات‌اسپات و DHCP و DNS و پنل را بالا می‌گذارد. ببینید [<span dir="ltr">`internal/privsvc/cut.go`</span>](https://github.com/Iman/caspian/blob/main/internal/privsvc/cut.go)
و کنشِ <span dir="ltr">`cut`</span> در [<span dir="ltr">`internal/panel/priv.go`</span>](https://github.com/Iman/caspian/blob/main/internal/panel/priv.go).

قطعِ ترافیک وضعیتِ زمانِ اجراست و هرگز روی دیسک نوشته نمی‌شود، پس کشیدنِ برق آن
را برمی‌گرداند. آزمون‌ها: <span dir="ltr">`TestCuttingClientTrafficLeavesTheWayBack`</span>،
<span dir="ltr">`TestACutIsNeverWrittenDown`</span>، <span dir="ltr">`TestACutDoesNotSurviveARestart`</span>،
<span dir="ltr">`TestForwardCut_StopsClientsAndKeepsThePanelReachable`</span>.

اینکه کدام‌یک از آن دو را باید زد، و هر کدام چه چیزی را برمی‌چیند، در بخشِ
«کنترل‌ها، و اینکه کدام را باید زد» در بالا آمده است.

### رابط را از هسته بازخوانی می‌کند، به‌جای اعتماد به یک فرایند

یک فرایندِ شروع‌شده شاهدِ کار کردنش نیست. این یک خرابیِ واقعی بود. سرآیندِ
[<span dir="ltr">`internal/privsvc/readback.go`</span>](https://github.com/Iman/caspian/blob/main/internal/privsvc/readback.go) ثبت کرده که در 2026-08-30 سرویس خودش را «در حال
کار» با یک هات‌اسپات روی <span dir="ltr">`wlan0`</span> ثبت کرد، در حالی که <span dir="ltr">`wlan0`</span> هنوز یک ایستگاه روی
شبکهٔ خانه بود. hostapd فرایندی زنده بود که سوکتِ کنترلش جواب نمی‌داد. گوشی‌ای در
همان اتاق یازده شبکه فهرست کرد که شبکهٔ ما میانشان نبود. و dnsmasq داشت به دستگاهِ
یک غریبه روی شبکهٔ محلیِ کسی دیگر با DHCPNAK جواب می‌داد.

تا وقتی <span dir="ltr">`netcfg.AssertHotspotInterfaceReleased`</span> ثابت نکند رابطِ هات‌اسپات آزاد
است، هیچ چیز اجازهٔ بایند شدن به آن را ندارد، و تا وقتی
<span dir="ltr">`AssertHotspotIsAccessPoint`</span> نقطهٔ دسترسی‌ای را بازخوانی نکند که نامِ مورد انتظار
را پخش می‌کند، هیچ چیز خودش را «در حال کار» گزارش نمی‌کند. آزمون‌ها:
<span dir="ltr">`TestNothingBindsToTheHotspotInterfaceUntilItIsProvedFree`</span>،
<span dir="ltr">`TestTheServiceDoesNotReportRunningUntilTheAccessPointReadsBackAsOne`</span>،
<span dir="ltr">`TestAnAccessPointBroadcastingAnotherNameIsNotOurs`</span>،
<span dir="ltr">`TestTheReleaseIsReadBackBeforeAnythingBindsAndTheAccessPointAfter`</span>.

### وای‌فایتان را پس می‌گیرید

هر تغییرِ شبکه همراه با وارونه‌اش **پیش از** انجام شدن در
<span dir="ltr">`/var/lib/caspian/netcfg.journal`</span> نوشته می‌شود، و خاموش کردن آن‌ها را به ترتیبِ
معکوس بازپخش می‌کند. این ثبت روی دیسک است نه در حافظه، پس فرایندی که کشته شود یا
قطعِ برق آن را از بین نمی‌برد. دستگاهی که وسطِ یک تغییر مرده باشد، پیش از آنکه به
دستگاه نگاه کند یا چیزِ تازه‌ای اعمال کند، این ثبت را بازپخش می‌کند.

تصاحبِ یک رابطِ وای‌فای گام‌به‌گام در دفترچه ثبت می‌شود. توالیِ رفت و وارونه‌هایش
روی دستگاهِ هدف اجرا شده و در <span dir="ltr">`PROVENANCE.md`</span> زیر عنوانِ
"The release sequence has been run on the target" ثبت شده‌اند. آن چهار فرمان
بیرون رفتند، و وارونه‌ها هشت ثانیه بعد دستگاه را با آدرسِ خودش به شبکهٔ خودش
برگرداندند.

یک تغییر عمداً وارونه ندارد، و <span dir="ltr">`TestPlan_InvariantsHoldOnEveryModelledMachine`</span>
تصدیق می‌کند که تنها همان یکی است: بالا آوردنِ رابطِ هات‌اسپات. پایین آوردنِ یک
رادیو هنگامِ خروج بدتر از بالا گذاشتنش است، چون وای‌فایِ خودِ دستگاه، و پنلی که
کاربر دارد می‌خواند، می‌توانند روی همان باشند.

سناریوها: "turning the switch off returns every change the box made"،
"a teardown replayed from the journal of a killed process undoes the same
changes"، "a box killed halfway through cleans up before it does anything else".
آزمون‌ها: <span dir="ltr">`TestJournal_RecordsInverseBeforeTheChange`</span>،
<span dir="ltr">`TestTeardown_ReplaysInExactReverseOrder`</span>،
<span dir="ltr">`TestRecover_UndoesAJournalLeftByAKilledProcess`</span>،
<span dir="ltr">`TestTheTakeoverReleasesTheInterfaceItSaysItWillRelease`</span>.

اگر وارونه‌ای شکست بخورد، وارونهٔ خودِ فایروال **نگه داشته می‌شود** و اجرا
نمی‌شود، پس دستگاهی که نتوانسته مسیرهایش را برگرداند، مسدودسازی‌اش را حفظ
می‌کند. آزمون: <span dir="ltr">`TestTheFirewallIsNotRemovedWhenAnEarlierInverseFailed`</span>.

### کانفیگِ پیست‌شده به هیچ صفحه‌ای، هیچ گزارشی و هیچ فایلِ خواندنی‌ای نمی‌رسد

هر چه این جریان تولید می‌کند برای یافتنِ کانفیگِ پیست‌شده جست‌وجو می‌شود:

- هر خطا، و هر پیامی که به کاربر نشان داده می‌شود
- خط‌های گزارشِ دستگاه
- توصیفی که پنل از کانفیگ می‌دهد
- تنظیماتِ ذخیره‌شده، آن‌طور که برای تشخیص رندر می‌شوند
- فایروالِ تولیدشده
- پیکربندیِ تولیدشدهٔ DHCP و DNS
- گزارشِ خودِ نرم‌افزار اتصال
- دفترچه، روی دیسک
- درخواستی که از پنلِ غیرممتاز به سرویسِ ممتاز می‌رود

در هیچ‌کدام نیست. و بررسی می‌شود که در آن دو جایی که باید باشد هست، تا آزمون
نتواند به این دلیل که کانفیگ اصلاً گم شده قبول شود.

سناریوها: "the pasted credential never reaches a screen, a log or a readable
file"، "the hotspot password reaches the access point and nothing else".
آزمون‌ها: <span dir="ltr">`TestPastedConfigNeverAppearsInAResponseOrALog`</span>،
<span dir="ltr">`TestFailedConfigPathsDoNotEchoTheInput`</span>، <span dir="ltr">`TestStartRequestRedactsItself`</span>،
<span dir="ltr">`TestNoCredentialReachesTheAdvancedView`</span>،
<span dir="ltr">`TestTheServerAddressNeverAppearsInADiagnosticLine`</span>.

### پنل از اینترنت چیزی نمی‌خواهد

هر شیوه‌نامه و اسکریپت و آیکنی که مرورگر بار می‌کند با <span dir="ltr">`go:embed`</span> داخل فایل
اجرایی کامپایل شده است. ببینید [<span dir="ltr">`internal/panel/assets.go`</span>](https://github.com/Iman/caspian/blob/main/internal/panel/assets.go). هیچ فونتِ وبی در کار
نیست: پشتهٔ فونتِ شیوه‌نامه تماماً از فونت‌های سیستمی است و فونت‌های
فارسی‌خوان اول می‌آیند.

دلیلِ حریم خصوصی این است که یک دارایی از راه دور، آدرسِ هر کسی را که پنل را باز
می‌کند به یک شخص ثالث می‌گوید. دلیلِ قوی‌تر در دسترس بودن است. پنل باید وقتی تونل
پایین است بار شود، و دقیقاً همان وقت است که کسی به آن نیاز دارد.

دو سازوکار، نه یکی. <span dir="ltr">`TestNoAssetReferencesAnExternalURL`</span> و
<span dir="ltr">`TestNoRenderedPageReferencesAnExternalURL`</span> دارایی‌ها و هر صفحهٔ رندرشده را برای
یافتنِ یک URL مطلق پویش می‌کنند. <span dir="ltr">`setSecurityHeaders`</span> با هر پاسخ
<span dir="ltr">`default-src 'none'`</span> می‌فرستد و هر منبعِ فهرست‌شده را روی <span dir="ltr">`'self'`</span> می‌گذارد، پس
مرورگر آن یکی را که از آزمون‌ها رد شده باشد رد می‌کند. هیچ کلاینتِ HTTP خروجی‌ای
در هیچ جای <span dir="ltr">`internal/panel`</span> بیرون از آزمون‌های خودش وجود ندارد.

پیکربندیِ تولیدشده هم هیچ resolver گوگلی را در هیچ جا نام نمی‌برد، و هیچ قاعدهٔ
<span dir="ltr">`geoip:`</span> یا <span dir="ltr">`geosite:`</span> به کار نمی‌برد، چون هر کدام یک دانلود را به محصولی
برمی‌گرداند که تمامِ داستانِ نصبش یک فایلِ اجراییِ وارسی‌شده است. آزمون‌ها:
<span dir="ltr">`TestNoGoogleAnywhereInGeneratedConfigs`</span>،
<span dir="ltr">`TestGoogleResolverIsRejectedAtTheSource`</span>. سناریو: "the box needs no download and
asks no Google server anything".

### دسترسیِ ممتاز جدا شده است

<span dir="ltr">`caspian serve --privileged`</span> با root اجرا می‌شود و مالکِ مسیرها، فایروال، نقطهٔ
دسترسی و نرم‌افزار اتصال است. یک فهرستِ کوتاه از کنش‌های نام‌دار را روی یک سوکتِ
یونیکس می‌پذیرد و هرگز فرمانی را که از ورودیِ کاربر ساخته شده باشد نمی‌پذیرد.
<span dir="ltr">`caspian serve --panel`</span> با حسابِ غیرممتازِ <span dir="ltr">`caspian`</span> اجرا می‌شود و مالکِ رابطِ وب
است و نه چیز دیگری. برای واژگان و قالبِ فریم، بخشِ «معماری» را در بالا ببینید.

رمزِ پنل با argon2id هش می‌شود. ببینید [<span dir="ltr">`internal/state/password.go`</span>](https://github.com/Iman/caspian/blob/main/internal/state/password.go). این یک رمزِ
محلی روی خودِ دستگاه است. هیچ حسابی در هیچ جای دیگری وجود ندارد.

### ساعت پیش از هر دست‌دادنی بررسی می‌شود

یک Pi ساعتِ باتری‌دار ندارد، و دو سازوکارِ جدا به ساعتِ دیواری وابسته‌اند.
REALITY آن را داخلِ دست‌دادن می‌نویسد، و اینکه xray-core چه کانفیگ‌هایی را
**می‌پذیرد** به تاریخ بستگی دارد. پس دستگاهی که ساعتش غلط بالا بیاید فقط در وصل
شدن شکست نمی‌خورد. کانفیگی را می‌پذیرد که همان فایلِ اجرایی، پس از درست شدنِ
ساعت، ردش می‌کند.

بررسی پیش از اعتبارسنجی و پیش از هر تلاشی اجرا می‌شود. ببینید
[<span dir="ltr">`internal/privsvc/clock.go`</span>](https://github.com/Iman/caspian/blob/main/internal/privsvc/clock.go)، که از <span dir="ltr">`Service.Start`</span> به‌عنوانِ گام 1 از
<span dir="ltr">`applyLocked`</span> صدا زده می‌شود. یک خطای متمایز بالا می‌آورد تا پنل کانفیگِ کاربر را
مقصر نداند. آزمون: <span dir="ltr">`TestClockFailureIsNotBlamedOnTheConfig`</span>.

### سه شکستِ کانفیگ از هم جدا می‌شوند

«نتوانستم آن لینک را بخوانم»، «خواندمش، و همان‌طور که هست قابلِ استفاده نیست»، و
«لینک سالم بود و سرور جواب نداد» سه کارِ متفاوت از کاربر می‌خواهند، و سومی از همه
رایج‌تر است. مقصر دانستنِ کانفیگ در وهلهٔ اول همان چیزی است که باعث می‌شود کسی
کانفیگی را که هرگز خراب نبوده دور بیندازد. پیش از خوانده شدنِ متنِ پیست‌شده به
هیچ چیزِ روی دستگاه دست زده نمی‌شود. سناریوها: "text that is not a link at all is
refused before anything is touched"، "a link the engine will not accept is told
apart from one that would not parse"، "a link whose server never answers is not
blamed on the link".

## چه چیزی را تضمین نمی‌کند

همین فهرست است که باید با دقت خوانده شود.

### DNS روی HTTPS در پورت 443 حمل می‌شود، مسدود نمی‌شود، و هیچ چیزِ اینجا نمی‌تواند ببیندش

DNS دستگاه‌ها روی پورت 53 در هر دو پروتکل به همین دستگاه بازهدایت می‌شود، نه
اینکه فقط اجازه داده شود. پس دستگاهی که resolver در خودش کدگذاری شده همین‌جا جواب
می‌گیرد، و اجازه ندارد بیرون برود تا به آنکه به آن گفته‌اند برسد. DNS روی TLS
روی 853 با یک TCP reset رد می‌شود، تا دستگاه به پورتِ بازهدایت‌شده عقب بنشیند.
DNS روی QUIC روی 853 دور ریخته می‌شود.

DNS روی HTTPS در پورت 443 از هر HTTPS دیگری قابلِ تشخیص نیست و مثل هر چیزِ دیگری
از تونل حمل می‌شود. دستگاهی که از آن استفاده می‌کند داخلِ تونل است و نشت نمی‌دهد.
همچنین نامرئی است. هیچ چیز در این پروژه، و هیچ چیز در بسترِ سخت‌افزاری، نمی‌تواند
آن را ببیند. این یک محدودیتِ طراحی است. این نکته در خودِ مجموعه‌قواعدِ تولیدشده،
در [<span dir="ltr">`docs/BEHAVIOUR.md`</span>](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md)، و در خروجیِ چاپ‌شدهٔ بررسیِ نشتِ DNS هم آمده، نه فقط
اینجا.

### IPv6 مسدود است، و مسیرِ IPv6 تمام نشده است

هیچ تونلِ IPv6 در کار نیست. دستگاهی که مسیرِ IPv6 سالمی داشته باشد آن را به IPv4
ترجیح می‌دهد و کلاً از تونل بیرون می‌زند، پس سیاستِ پیش‌فرض مسدود کردن است. چهار
چیز این را نگه می‌دارد. <span dir="ltr">`IPv6Block`</span> پیش‌فرضِ <span dir="ltr">`netcfg.DefaultOptions`</span> است. دستگاه
IPv6 را عبور نمی‌دهد. فایروال، IPv6 عبوری روی هات‌اسپات را در هر دو جهت drop
می‌کند. و اعلان‌های روتر به سمتِ هات‌اسپات دور ریخته می‌شوند، تا دستگاهی نتواند
به خودش آدرس بدهد. سناریو: "clients are never offered the IPv6 the tunnel cannot
carry".

<span dir="ltr">`IPv6Forward`</span> به‌عنوانِ یک گزینه وجود دارد و کامنتِ خودش می‌گوید آن را تنظیم
نکنید. نشان داده نشده که ورودیِ TUN نرم‌افزار اتصال روی دستگاهِ هدف IPv6 را حمل
کند. این گزینه عمداً هیچ قاعدهٔ اجازه‌ای هم به زنجیرهٔ forward اضافه نمی‌کند:
قاعده‌های IPv4 در هر دو جهت زیرشبکهٔ هات‌اسپات را نام می‌برند، هیچ پیشوندِ v6ای در
نقشه نیست که نامش برده شود، و قاعده‌ای که فقط نامِ دو رابط را بگوید هر آدرسِ مبدأیی
را که کاربر بنویسد می‌پذیرد. <span dir="ltr">`TestRuleset_NoUnconstrainedIPv6AcceptInForward`</span> همین
خط را نگه می‌دارد.

**«مسدود» دربارهٔ مسیریابی است و نه دربارهٔ DNS، و این تفاوت مهم است.** پرسشِ AAAA
از یک دستگاهِ وصل‌شده نه حذف می‌شود و نه خالی پاسخ داده می‌شود. به موتور می‌رود، از
داخلِ تونل می‌گذرد، و با رکوردهای AAAA واقعی برمی‌گردد، چون سندِ موتور <span dir="ltr">`UseIP`</span> را
می‌خواهد و dnsmasq هیچ <span dir="ltr">`filter-AAAA`</span> تنظیم نمی‌کند. پس دستگاه آدرس‌های IPv6ای را
یاد می‌گیرد که هیچ راهی برای رسیدن به آن‌ها ندارد و به IPv4 برمی‌گردد.

تا وقتی هیچ چیز نتواند به یک دستگاه آدرسِ v6 بدهد این بی‌ضرر است، و اولین چیزی است
که اگر روزی چیزی چنین آدرسی بدهد بی‌ضرر نمی‌ماند، چون دستگاهی که مسیرِ v6 کارآمد
داشته باشد پاسخِ AAAA را ترجیح می‌دهد و از مسیری بیرون می‌رود که این جعبه حملش
نمی‌کند. این را اینجا نوشته‌ایم تا غافلگیری نباشد، و
<span dir="ltr">`TestAAAAQueriesAreAnsweredAndNotSuppressed`</span> هر دو نیمه را پین می‌کند تا تغییرش
ناچار یک تصمیم باشد.

**بسترِ سخت‌افزاری اصلاً نمی‌تواند به IPv6 نمره بدهد، پس هیچ نتیجهٔ IPv6 از آن
معنایی ندارد.** [<span dir="ltr">`test/hardware/README.md`</span>](https://github.com/Iman/caspian/blob/main/test/hardware/README.md) زیر عنوانِ
"What this vantage cannot grade: IPv6" ثبت کرده که گوشی فقط یک آدرسِ link-local
دارد، که <span dir="ltr">`ip -6 route show default`</span> هم روی گوشی و هم روی Pi خالی است، و که اتصال
به یک آدرسِ صریحِ IPv6 جوابِ "Network is unreachable" می‌گیرد. روی آن شبکهٔ محلی
اصلاً IPv6 وجود ندارد، پس یک بررسیِ نشتِ IPv6 که آنجا اجرا شود بدون آنکه دستگاه
کاری کرده باشد قبول می‌شود. هر نتیجهٔ سخت‌افزاری‌ای که این پروژه دارد یک نتیجهٔ
IPv4 است. هر کسی که این را روی شبکه‌ای با IPv6 سالم اجرا می‌کند باید آن را یک
پرسشِ تازه بداند، نه پرسشی که پوشش داده شده، و باید انتظار داشته باشد که خودش
آزمون را بنویسد، نه اینکه آزمونی را روشن کند.

### ترافیکِ خودِ دستگاه، عمداً بیرون از قولِ fail-closed است

آن قول دربارهٔ **ترافیکِ عبوریِ دستگاه‌ها** است. اتصالِ خودِ این دستگاه به سرورِ
شما باید مستقیم به رابطِ اینترنت برسد وگرنه اصلاً تونلی وجود ندارد، و
[<span dir="ltr">`docs/2026-08-29-design.md`</span>](https://github.com/Iman/caspian/blob/main/docs/2026-08-29-design.md) بخش 7 به همین دلیل ترافیکِ خودِ دستگاه را بیرونِ آن
تضمین می‌گذارد.

کلیدِ قطع روی زنجیرهٔ output آن را تنگ‌تر می‌کند و نمی‌بنددش. مجموعه‌قواعدِ
تولیدشده باقی‌ماندهٔ آن را در سرآیندِ خودش می‌نویسد. DNS یک سوراخ است: هر چیزی
روی دستگاه هنوز می‌تواند روی پورت 53 به شبکه برسد، و نامِ میزبانِ سرور پیش از
آنکه تونلی وجود داشته باشد به‌صورتِ آشکار روی شبکهٔ محلی ترجمه می‌شود. هیچ‌کدام
نشتِ ترافیکِ دستگاه‌ها نیست، و کلیدِ قطع هیچ‌کدام را بدتر نمی‌کند.

سیاستِ زنجیرهٔ input برابرِ <span dir="ltr">`accept`</span> است، آن هم عمدی و آن هم نوشته‌شده در
مجموعه‌قواعد. نسخهٔ قبلی آن را <span dir="ltr">`drop`</span> گذاشته بود، و <span dir="ltr">`PROVENANCE.md`</span> ثبت کرده وقتی
روی دستگاهِ هدف اندازه‌گیری شد چه شد. هر اتصالِ ورودیِ تازه رد شد و SSH از جواب
دادن افتاد، در حالی که نشستِ از پیش باز همچنان کار می‌کرد، و این روی یک دستگاهِ
بدون صفحه‌نمایش از یک کرش قابلِ تشخیص نیست. تنها جایی که زنجیرهٔ input چیزی را
محدود می‌کند سمتِ هات‌اسپات است، جایی که یک دستگاهِ وصل‌شده به DHCP و DNS و پنل و
ICMP echo می‌رسد و به هیچ چیزِ دیگری روی دستگاه نمی‌رسد.

### جداسازیِ دستگاه‌ها یک قاعده است، نه یک اندازه‌گیری

مجموعه‌قواعد شاملِ <span dir="ltr">`iifname "wlan0" oifname "wlan0" drop`</span> است. حضورِ این قاعده
بررسی می‌شود. کار کردنش بررسی نمی‌شود.

### هیچ چیز در این مخزن آدرسِ خروجی نمی‌گیرد

<span dir="ltr">`test/tunnel`</span> بایت‌های واقعی را از دلِ یک سرور واقعیِ xray-core عبور می‌دهد، و هر
چه در آن است روی loopback است، پس هیچ آدرسِ خروجی‌ای نمی‌گیرد و نمی‌تواند
بگیرد. <span dir="ltr">`test/bdd`</span> نه شبکه دارد، نه رادیو، نه root و نه دستگاهِ تونل. نرم‌افزار
اتصالِ واقعی را در فرایند و از راهِ بارگذارندهٔ واقعیِ کانفیگ اجرا می‌کند، پس
«نرم‌افزار اتصال این کانفیگ را پذیرفت و شروع کرد» همان معنایی را می‌دهد که
می‌گوید، اما ورودیِ تونل خاموش است.

پس هیچ چیز در این مخزن استانداردِ خودِ این پروژه را برای «کار می‌کند» نامیدنِ
چیزی برآورده نمی‌کند. [<span dir="ltr">`docs/BEHAVIOUR.md`</span>](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md) با بخشی به نامِ
"What this suite does not prove" تمام می‌شود که فهرست می‌کند چه چیزی هنوز بدهکار
است. آن را بخشی از مجموعهٔ آزمون بخوانید.

### هیچ چیز فایروال را پس از بار شدن دوباره بررسی نمی‌کند

نقصِ D1 در پایین را ببینید. اگر چیزی جدول را در حالی که دستگاه در حال کار است
پاک کند، دستگاه به عبور دادن ادامه می‌دهد، پنل به گزارشِ «متصل» ادامه می‌دهد، و
هیچ چیز متوجه نمی‌شود.

### هیچ چیز رابطِ اینترنت را نمی‌پاید

جابه‌جا شدنِ اینترنت، چون کابلی کشیده شد یا اجاره‌ای جور دیگری تمدید شد، تغییری
است که هیچ چیز برایش بلند شکست نمی‌خورد. مسیرِ سنجاق‌شده به سرور هنوز وجود دارد و
هنوز به آدرسی اشاره می‌کند که دیگر راهِ بیرون رفتن نیست. دستگاه متوجه نمی‌شود، و
تونل تا وقتی کسی دوباره کلید را بزند متوقف می‌ماند.

هزینهٔ این، در دسترس بودن است نه حریم خصوصی. ترافیکِ دستگاه‌ها در تمامِ مدت مسدود
می‌ماند، چون سیاستِ forward برابرِ drop است و هر accept در آن نامِ تونل را
می‌برد. <span dir="ltr">`netcfg.WatchUplink`</span> و <span dir="ltr">`Plan.RederiveForUplink`</span> وجود دارند و کار
می‌کنند، و هیچ کدِ منتشرشده‌ای هیچ‌کدام را صدا نمی‌زند.
<span dir="ltr">`TestNothingInTheApplianceWatchesTheUplink`</span> همان چیزی است که جلوی برگشتنِ جملهٔ
وارونه به داخلِ اسناد را می‌گیرد، جایی که تا 2026-08-30 ایستاده بود.

### حالت B هرگز روی سخت‌افزار واقعی اجرا نشده است

هر فیکسچرِ حالت B نوشته شده است. <span dir="ltr">`PROVENANCE.md`</span> ثبت کرده که دستگاهِ هدف یک رادیو
دارد و هیچ آداپتور USB ای ندارد، پس چیدمانی که این محصول به مردم می‌گوید برایش
آداپتور بخرند، در برابرِ بایت‌هایی اثبات شده که هیچ‌کس اندازه‌شان نگرفته.

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

[Architecture](https://github.com/Iman/caspian/wiki/Architecture) | [Panel-and-Configuration](https://github.com/Iman/caspian/wiki/Panel-and-Configuration) | [Troubleshooting](https://github.com/Iman/caspian/wiki/Troubleshooting)

</div>
