<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Architecture) | [فارسی](https://github.com/Iman/caspian/wiki/Architecture.fa) | [Русский](https://github.com/Iman/caspian/wiki/Architecture.ru) | [中文](https://github.com/Iman/caspian/wiki/Architecture.zh) | [العربية](https://github.com/Iman/caspian/wiki/Architecture.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Architecture.tr) | [اردو](https://github.com/Iman/caspian/wiki/Architecture.ur)

</div>

<div dir="rtl" align="right">

<a id="architecture-and-data-flow"></a>
# معماری و جریان داده ها



[ویکی کاسپین](https://github.com/Iman/caspian/wiki/Home.fa)

> این راهنما از README موجود می آید. اندازه گیری های آن تاریخ اصلی خود را حفظ می کند. این حرکت مستندسازی اجرای آزمایشی جدیدی را گزارش نمی‌کند.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="architecture"></a>
## معماری

<a id="two-processes-one-binary"></a>
### دو فرآیند، یکی باینری

یک باینری در دو نقش اجرا می شود که توسط دستور فرعی انتخاب می شوند. شکاف وجود دارد به طوری که الف
خطا در بخشی که ورودی کاربر را تجزیه و تحلیل می کند و HTTP را ارائه می دهد یک نقص در قسمت نیست
بخشی که ریشه دارد [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md)، "دو فرآیند، یک باینری" است
بیانیه ثابت آن

```mermaid
flowchart LR
    subgraph device["A device joined to the hotspot"]
        BR["Browser<br/>port 8088 on the hotspot address"]
    end

    subgraph panelproc["caspian serve --panel, runs as the caspian account"]
        PANEL["internal/panel<br/>routes, sessions, wording, rendering"]
        STATE["internal/state<br/>the only writer of state.json"]
        LINK1["internal/link<br/>parse the pasted share link"]
        ENG1["internal/engine<br/>Validate only, opens no socket"]
    end

    subgraph privproc["caspian serve --privileged, runs as root"]
        SVC["internal/privsvc<br/>Service.Start, Stop, Cut, Restore, Recover"]
        XCFG["internal/xcfg<br/>compose the engine document"]
        NETCFG["internal/netcfg<br/>routes, nftables, the teardown journal"]
        HOT["internal/hotspot<br/>hostapd and dnsmasq"]
        ENG2["internal/engine<br/>xray-core, in this process"]
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

[`cmd/caspian/main.go`](https://github.com/Iman/caspian/blob/main/cmd/caspian/main.go) دو نقش را در متن استفاده خود چاپ می کند:

caspian serve -- root privileged: routes, firewall, access point, engine
caspian serve --panel the caspian user: پنل وب، هیچ چیز ممتازی ندارد

<a id="the-socket-and-why-the-vocabulary-is-closed"></a>
### سوکت، و چرا واژگان بسته است

[`internal/panel/priv.go`](https://github.com/Iman/caspian/blob/main/internal/panel/priv.go) قاعده ای را بیان می کند که کل تقسیم برای آن وجود دارد: "الف
کمک کننده ممتازی که یک مسیر و لیست آرگومان را از مشتری خود می گیرد، نیست
یک مرز؛ این راهی برای اجرای هر چیزی به عنوان روت است." نقطه ویرگول مال آنهاست. را
جمله دقیقاً نقل شده است، زیرا نقل یک قاعده، قاعده نیست.

بنابراین پانل نمی تواند "اجرای این" را بیان کند. فقط می تواند یکی از هشت عمل را نام برد،
و طرف ممتاز تصمیم می گیرد که منظور هر کدام چیست. `panel.Actions` این است
مجموعه بسته شد، و اگر یک روش وجود داشته باشد، `TestActionVocabularyMatchesTheInterface` با شکست مواجه می شود
بدون نام در لیست به رابط اضافه شده است.

| اقدام | کاری که طرف ممتاز انجام می دهد | دستگاه را عوض می کند |
|---|---|---|
| `detect` | رابط ها، محدودیت های رادیو و زیرشبکه انتخابی را گزارش کنید | نه |
| `status` | فاز موتور، هات اسپات و قطع شدن ترافیک را گزارش دهید | نه |
| `start` | تونل و هات اسپات را بالا بیاورید | بله |
| `stop` | آنها را پایین بیاورید و ژورنال پارگی را دوباره پخش کنید | بله |
| `recover` | توقف کنید، ژورنال را دوباره پخش کنید، سپس دوباره از همان درخواست شروع کنید | بله |
| `engine-log` | خطوط اخیر موتور را که قبلاً ویرایش شده است برگردانید | نه |
| `cut` | ترافیک ارسال‌شده مشتری را رها کنید و بقیه موارد را در حال اجرا بگذارید | بله |
| `restore` | ترافیک ارسال شده مشتری را برگردانید | بله |

یک درخواست، یک پاسخ، یک اتصال. یک پیام یک 4 بایتی بزرگ است
طول و به دنبال آن تعداد زیادی بایت JSON. طول در برابر بررسی می شود
`maxFrameBytes` قبل از تخصیص یا تجزیه هر چیزی، بنابراین یک پیام بزرگ
هزینه چهار بایت و یک رد. فیلدهای JSON ناشناخته رد می شوند
نادیده گرفته شده است. `protocolVersion` در هر درخواست بررسی می شود. بنابراین یک پانل از یک
رهایی از صحبت کردن با یک سرویس ممتاز از طرف دیگر، یک امتناع نامی دریافت می کند،
به جای یک فیلد که در سکوت به عنوان مقدار صفر آن رمزگشایی می شود.

هیچ چیز در مسیر شکست به عقب بر نمی گردد به جز یک کلمه: `panel.Fault` از
یک مجموعه بسته، یا یک `privsvc.Refusal` از مجموعه بسته دوم. مال موتوره
متن خطا مطالب کلیدی کاربر را جاسازی می کند، بنابراین در ممتاز وارد می شود
طرف و افتاد. هیچ فیلدی در مورد پاسخی که می تواند در آن سفر کند وجود ندارد.

<a id="who-owns-which-package"></a>
### چه کسی صاحب کدام بسته است

```mermaid
flowchart TB
    LINK["internal/link<br/>share link in, one outbound out.<br/>Carries no credential in an exported field"]
    XCFG["internal/xcfg<br/>everything around the outbound:<br/>TUN inbound, SOCKS, local DNS, routing"]
    ENGINE["internal/engine<br/>starts and stops xray-core.<br/>Redacts every line on the way in"]
    NETCFG["internal/netcfg<br/>plans the machine, generates the ruleset,<br/>journals the inverse of every change"]
    HOTSPOT["internal/hotspot<br/>renders and supervises hostapd and dnsmasq.<br/>Detects no interface, queries no radio"]
    STATE["internal/state<br/>state.json, atomically, 0600"]
    PANEL["internal/panel<br/>the web interface and the fault vocabulary"]
    PRIVSVC["internal/privsvc<br/>the order of the steps, and the readbacks"]

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

`internal/privsvc` `StartRequest.ConfigJSON` را با `internal/link` دوباره تجزیه می کند
به جای اعتماد به پنل که این کار را انجام داده است. اینترنت رو هم چک میکنه
رابط در برابر مسیر پیش فرض خود این دستگاه، رابط نقطه اتصال
در برابر خروجی `iw list` خود این دستگاه و کانال در برابر آنچه که
رادیو به عنوان قابل استفاده گزارش شد.

<a id="where-state-lives-and-who-writes-it"></a>
### ایالت کجا زندگی می کند و چه کسی آن را می نویسد

دو نویسنده، دو فایل، بدون فایل مشترک. هیچ یک از فرآیندهای دیگر را نمی نویسد، بنابراین
هیچ قفل و به روز رسانی گم شده ای برای محافظت وجود ندارد. [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md)، "چه کسی
می نویسد چه»، تصمیم را ثبت می کند و پیش نویس قبلی آن را معکوس می کند.

```mermaid
flowchart TB
    subgraph panelowns["Written only by caspian serve --panel"]
        SJ["/var/lib/caspian/state.json<br/>0600 caspian. Holds the pasted config<br/>and the hotspot passphrase"]
    end

    subgraph privowns["Written only by caspian serve --privileged"]
        JN["/var/lib/caspian/netcfg.journal<br/>0600 root. The inverse of every change,<br/>written before the change"]
        HC["/run/caspian/hostapd.conf<br/>0600 root, tmpfs, rewritten every start"]
        DC["/run/caspian/dnsmasq.conf<br/>0600 root, tmpfs, rewritten every start"]
    end

    subgraph nofile["Held in memory and written to no file"]
        CUT["the cut"]
        EVT["the panel's event list"]
        RING["the engine log ring"]
    end
```

طرف ممتاز اصلاً هیچ پرونده ایالتی را نمی خواند. هر چیزی که نیاز دارد وارد می شود
درخواست شروع `TestPrivsvcReadsNoStateFile` منبع خود بسته را اسکن می کند
و اگر یک مورد را بخواند، شکست می خورد، که نظر ارائه نمی کرد.

جدول کامل مسیرها، حالت‌ها و مالکان در [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md) است. پورت ها هستند
در آنجا نیز ثابت شد: 53 برای DNS مشتری در هات اسپات، 5354 در Loopback برای
شنونده DNS موتور، 8088 برای پنل، 10808 در Loopback برای
تشخیص SOCKS ورودی.

<a id="how-data-flows"></a>
## نحوه جریان داده ها

<a id="a-pasted-share-link-becomes-a-running-tunnel"></a>
### پیوند اشتراک گذاری چسبانده شده به یک تونل در حال اجرا تبدیل می شود

`startNow` در [`internal/panel/handlers.go`](https://github.com/Iman/caspian/blob/main/internal/panel/handlers.go) سفارش را مستند می کند و سفارش
چه چیزی سه شکست پیکربندی را از هم جدا می کند. هیچ چیز روی دستگاه لمس نمی شود
تا حالت 1 و حالت 2 هر دو بگذرند.

```mermaid
sequenceDiagram
    autonumber
    participant U as The person at the panel
    participant PA as internal/panel
    participant LK as internal/link
    participant EN as internal/engine
    participant PS as internal/privsvc, root
    participant NC as internal/netcfg
    participant HS as internal/hotspot

    U->>PA: POST /power, on=1
    PA->>LK: link.Parse of the stored text
    Note over LK: State 1. It did not parse.<br/>The user has to fix the text.
    LK-->>PA: a Link that holds no credential in any exported field
    PA->>LK: Link.XrayConfig
    LK-->>PA: one outbound, tagged proxy, nulls removed
    PA->>EN: engine.Validate
    Note over EN: State 2. Read, and unusable as written.<br/>No socket opens. Nothing is dialled.
    PA->>PS: StartRequest over priv.sock
    PS->>PS: clock floor, re-parse, validate against this machine
    PS->>NC: Detect, then PlanNetwork
    PS->>PS: xcfg.Build, then engine.Validate again
    PS->>NC: Apply PreEngineSteps. The firewall is first.
    PS->>NC: AssertHotspotInterfaceReleased
    PS->>EN: Engine.Start. The tunnel device appears here.
    PS->>NC: Apply PostEngineSteps. Each needs the tunnel or engine listener.
    PS->>HS: Supervisor.Start: hostapd, then dnsmasq
    PS->>NC: AssertHotspotIsAccessPoint
    PS->>PS: probe the server
    Note over PS: State 3. The link was fine and the<br/>server did not answer. No rollback:<br/>the box is fully configured and blocking.
    PS-->>PA: nil, or one panel.Fault
```

سه جزئیات در آن دنباله تحمل بار هستند.

سند موتور دو بار به دلایل مختلف تنظیم می شود. `internal/link`
خروجی را تولید می کند و هیچ چیز دیگری. `internal/xcfg` همه چیز را تولید می کند
اطراف آن: ورودی TUN که ترافیک مشتری به آن می رسد، حلقه بک SOCKS
ورودی توسط عیب‌یابی و پروکسی موقت سیستم macOS، DNS محلی استفاده می‌شود
شنونده، خط مشی حل کننده و قوانین مسیریابی.
هیچ کدام از آن چیزی که تماس گیرنده فرستاده گرفته نمی شود.

شروعی که تا حدی با شکست مواجه شود، کاملاً لغو می شود. مجله قبلا
معکوس هر تغییری را نگه می دارد که قبل از رسیدن تغییر به دیسک نوشته شده است
هسته شروعی که با شکست مواجه می شود، دستگاه را همانگونه که پیدا شده است، رها می کند.

سروری که جواب نمی دهد یک جعبه نیمه کاربردی نیست. هر تغییری موفقیت آمیز بود،
فایروال فعال است و ترافیک مشتری ارسال شده مسدود شده است زیرا
تونل چیزی حمل نمی کند بنابراین عیب گزارش می شود و چیزی پاره نمی شود.

<a id="the-network-path-of-a-client-packet"></a>
### مسیر شبکه بسته مشتری

```mermaid
flowchart TB
    DEV["A joined device<br/>address from dnsmasq"] --> IF["The hotspot interface"]
    IF --> PRE["nft chain prerouting, type nat<br/>DNS on port 53 is redirected here"]
    PRE --> ROUTE{"Routing decision<br/>ip rule from the hotspot subnet<br/>lookup table 8410"}
    ROUTE -->|"tunnel route present"| TOTUN["oif is the tunnel device<br/>default route in table 8410"]
    ROUTE -->|"tunnel route withdrawn"| TOUP["oif is the uplink"]
    TOTUN --> FW1["nft chain forward, policy drop"]
    TOUP --> FW2["nft chain forward, policy drop"]
    FW1 -->|"iifname hotspot oifname tunnel<br/>ip saddr the hotspot subnet, accept"| POST["nft chain postrouting<br/>deliberately empty, no masquerade"]
    FW2 -->|"iifname hotspot oifname uplink, drop<br/>the leak block, first rule in the chain"| DROP["dropped"]
    POST --> TUN["The tunnel device<br/>a userspace netstack in the engine"]
    TUN --> OB["the outbound tagged proxy"]
    OB --> UP["The uplink<br/>a pinned host route to the server"]
    UP --> SRV["Your server"]
```

بلوک نشت فقط هات اسپات و لینک بالا را نام می برد. نمی تواند کار را متوقف کند
هنگامی که تونل می رود، زیرا در آن اشاره ای به تونل نمی شود. هر قانون که
اجازه می دهد که ترافیک مشتری تونل را نامگذاری کند، بنابراین این قوانین مطابقت ندارند و
سیاست همه چیز را رها می کند.

هر رابط با نام و هرگز با فهرست مطابقت داده می شود. یک شاخص زمانی حل می شود که
مجموعه قوانین بارگذاری می شود، بنابراین مجموعه قوانینی که تونل را بر اساس شاخص نامگذاری می کند، نمی تواند در حالی که بارگذاری شود
تونل خراب است، دقیقاً زمانی که باید راه اندازی شود.

زنجیره postrouting عمدا خالی است. بالماسکه به سمت بالا لینک است
خط واحدی که دستگاه را بی سر و صدا به یک روتر معمولی تبدیل می کند.

<a id="what-the-tunnel-disappearing-does-to-that-path"></a>
### ناپدید شدن تونل با آن مسیر چه می کند

```mermaid
flowchart TB
    GONE["The tunnel stops carrying traffic"] --> Q{"Does the device still exist?"}
    Q -->|"device removed"| WD["The kernel withdraws every route through it"]
    WD --> FB["Client traffic falls back to the main table<br/>and heads for the uplink"]
    FB --> LB["The leak block matches: iifname hotspot oifname uplink, drop"]
    Q -->|"device persists with nothing servicing it"| ENTER["Traffic enters the tunnel device"]
    ENTER --> NOWHERE["Nothing reads it. It goes no further."]
    LB --> SAFE["No client traffic leaves"]
    NOWHERE --> SAFE
```

کدام شاخه اتفاق می افتد تسویه حساب نمی شود. [`internal/netcfg/testdata/PROVENANCE.md`](https://github.com/Iman/caspian/blob/main/internal/netcfg/testdata/PROVENANCE.md)
مشاهده ای از هدف را در 30/08/2026 ثبت می کند: `xray0` در
لیست دستگاه های NetworkManager با خاموش بودن سرویس، به عنوان
`connected (externally)`. هیچ چیز در اینجا مشخص نکرد که چرا، و موتور نیست
کد این پروژه هیچ یک از شاخه ها نشت نمی کند و هیچ کدام به دانستن کدام یک بستگی ندارد
یکی اتفاق می افتد به همین دلیل است که بلوک فقط برای نامگذاری هات اسپات و the نوشته شده است
آپلینک

<a id="the-dns-path-which-is-not-the-traffic-path"></a>
### مسیر DNS که مسیر ترافیک نیست

این قسمتی است که مردم اشتباه می کنند. سؤال DNS یک مشتری صرفاً نیست
مجاز است. گرفته می شود.

```mermaid
flowchart TB
    ASK["A joined device asks whatever resolver it was told to use,<br/>or one hardcoded into it, on port 53"]
    ASK --> RD["nft prerouting on the hotspot:<br/>udp dport 53 and tcp dport 53 redirect to :53<br/>The destination address is rewritten to this box"]
    RD --> DM["dnsmasq, bound to the hotspot interface<br/>/run/caspian/dnsmasq.conf"]
    DM -->|"its only permitted upstream is a loopback address"| LD["the engine's DNS listener<br/>127.0.0.1:5354, inbound tag local-dns-in"]
    LD --> R1["rule ruleTagLocalDNS<br/>inboundTag local-dns-in, outbound dns-out"]
    R1 --> APP["the engine's DNS app<br/>resolvers from internal/xcfg/resolvers.go"]
    APP --> R2["rule ruleTagResolvers<br/>inboundTag resolver-in, outbound proxy.<br/>Above the private-address rule"]
    R2 --> OB["the outbound tagged proxy"]
    OB --> EXIT["the resolver chain, reached from the far end of the tunnel"]
```

چهار خاصیت از آن زنجیره، هر کدام با چیزی که آن را نگه می دارد.

تغییر مسیر، مقصد را بازنویسی می‌کند، بنابراین دستگاهی با یک حل‌کننده کدگذاری شده است
در اینجا به جای اینکه اجازه داده شود به کسی که به آن گفته شده است برسد، پاسخ داده می شود
استفاده کنید. سناریو: "یک مشتری نمی تواند به حل کننده ای که خودش انتخاب کرده است برسد".

پیشنهاد DHCP یک بار این کادر را نامگذاری می کند و هیچ حل کننده دیگری وجود ندارد. که ارزش خودش را دارد
این سناریو به این دلیل است که اشتباه گرفتن آن نامرئی است: تغییر مسیر باعث بازنویسی مجدد می شود
به هر حال بسته ها، بنابراین هیچ چیز روی سیم اشتباه به نظر نمی رسد. سناریو: "جعبه
خود را به عنوان حل کننده عرضه می کند و هرگز از دیگری نام نمی برد».

`internal/hotspot` هرگونه dnsmasq بالادستی را که آدرس حلقه بک نباشد، رد می کند.
یک هدف غیرحلقه‌ای، درخواستی است که جعبه را خارج از تونل می‌گذارد
هر نامی که هر مشتری می خواهد شنونده موتور اونجا جواب میده
و `TestLocalDNSDefaultMatchesTheHotspotUpstream` در صورت جابجایی دو پورت از کار می افتد.
[`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md) جفتی که بی سر و صدا می شکند را فراخوانی می کند: اگر این دو
رانش، هر دستگاه متصل شده در حالی که هات اسپات و تونل هر دو حل نمی شوند
سالم به نظر برسند

قاعده‌ای که درخواست‌های خود حل‌کننده را به داخل تونل ارسال می‌کند، بالاتر از آن قرار دارد
قانونی که آدرس های خصوصی را مستقیما ارسال می کند. بنابراین یک حل کننده در یک آدرس خصوصی است
هنوز از طریق تونل به جای شبکه محلی رسیده است.
`TestLocalDNSQueriesCannotFallOutToTheUplink` و `TestPrivateRangesRouteDirect`
دو نیمه را نگه دارید

خود زنجیره حل‌کننده سه اپراتور در سه حوزه قضایی است: Quad9
سرویس فیلتر شده، نوع Cloudflare FAMILY و CleanBrowsing Security.
[`internal/xcfg/resolvers.go`](https://github.com/Iman/caspian/blob/main/internal/xcfg/resolvers.go) چرایی هر کدام و تقریباً یکسان را ثبت می کند
آدرس همان اپراتور عمدا نیست. هیچ Google Resolver ظاهر نمی شود
در هر پیش‌فرض، و `TestNoGoogleAnywhereInGeneratedConfigs` هر کدام را اسکن می‌کند
سند تولید شده برای یک

سایر پورت ها مدیریت می شوند و یکی از آنها نمی تواند:

```mermaid
flowchart LR
    DOT["DNS over TLS<br/>tcp 853"] --> REJ["reject with tcp reset,<br/>so the device falls back to port 53"]
    DOQ["DNS over QUIC<br/>udp 853"] --> DRP["drop"]
    DOH["DNS over HTTPS<br/>port 443"] --> CAR["carried through the tunnel like any HTTPS.<br/>Not a leak. Not visible to anything here."]
```



<!-- Caspian guide navigation -->

راهنماهای کاسپین: [راه اندازی و پروتکل های پشتیبانی شده](https://github.com/Iman/caspian/wiki/Home.fa) · [جعل SNI برای دور زدن DPI: راه اندازی و محدودیت ها](https://github.com/Iman/caspian/wiki/SNI-Spoofing.fa).

</div>


<!-- English-source-sha256: 07a2e0584db74a162eb7938428d733054b00788748889213f96e0e0679d0f650 -->
