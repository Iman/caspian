<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Architecture) | [فارسی](https://github.com/Iman/caspian/wiki/Architecture.fa) | [Русский](https://github.com/Iman/caspian/wiki/Architecture.ru) | [中文](https://github.com/Iman/caspian/wiki/Architecture.zh) | [العربية](https://github.com/Iman/caspian/wiki/Architecture.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Architecture.tr) | [اردو](https://github.com/Iman/caspian/wiki/Architecture.ur)

</div>

<div dir="rtl" align="right">

<a id="architecture-and-data-flow"></a>
# فن تعمیر اور ڈیٹا کا بہاؤ



[Caspian ویکی](https://github.com/Iman/caspian/wiki/Home.ur)

> یہ گائیڈ موجودہ README سے آتا ہے۔ اس کی پیمائش اپنی اصل تاریخوں کو برقرار رکھتی ہے۔ یہ دستاویزی اقدام نئے ٹیسٹ رن کی اطلاع نہیں دیتا ہے۔
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="architecture"></a>
## فن تعمیر

<a id="two-processes-one-binary"></a>
### دو عمل، ایک بائنری

ایک بائنری دو کرداروں میں چلتی ہے، جسے ذیلی کمانڈ کے ذریعے منتخب کیا جاتا ہے۔ تقسیم موجود ہے تاکہ a
اس حصے میں غلطی جو صارف کے ان پٹ کو پارس کرتا ہے اور HTTP کو پیش کرتا ہے اس میں کوئی غلطی نہیں ہے۔
وہ حصہ جو جڑ رکھتا ہے۔ [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md)، "دو عمل، ایک بائنری"، ہے۔
اس کا مقررہ بیان۔

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

[`cmd/caspian/main.go`](https://github.com/Iman/caspian/blob/main/cmd/caspian/main.go) اپنے استعمال کے متن میں دو کرداروں کو پرنٹ کرتا ہے:

caspian serve --privileged root: راستے، فائر وال، ایکسیس پوائنٹ، انجن
caspian serve --panel the caspian user: ویب پینل، کچھ بھی مراعات یافتہ نہیں۔

<a id="the-socket-and-why-the-vocabulary-is-closed"></a>
### ساکٹ، اور الفاظ کیوں بند ہیں

[`internal/panel/priv.go`](https://github.com/Iman/caspian/blob/main/internal/panel/priv.go) اس اصول کو بیان کرتا ہے جس کے لیے پوری تقسیم موجود ہے: "A
مراعات یافتہ مددگار جو راستہ اختیار کرتا ہے اور اپنے مؤکل سے دلیل کی فہرست نہیں ہے۔
ایک حد؛ یہ کسی بھی چیز کو جڑ کے طور پر چلانے کا ایک طریقہ ہے۔" سیمی کالون ان کا ہے۔ دی
جملہ بالکل ٹھیک نقل کیا گیا ہے، کیونکہ اصول کا پیرا فریس اصول نہیں ہے۔

لہذا پینل "اس کو چلائیں" کا اظہار نہیں کرسکتا۔ یہ آٹھ اعمال میں سے صرف ایک کا نام دے سکتا ہے،
اور مراعات یافتہ فریق فیصلہ کرتا ہے کہ ہر ایک کا کیا مطلب ہے۔ `panel.Actions` وہ ہے۔
بند سیٹ، اور `TestActionVocabularyMatchesTheInterface` ناکام ہوجاتا ہے اگر کوئی طریقہ ہے۔
فہرست میں نام کے بغیر انٹرفیس میں شامل کیا گیا۔

| ایکشن | مراعات یافتہ فریق کیا کرتا ہے۔ | مشین بدلتا ہے۔ |
|---|---|---|
| `detect` | انٹرفیس، ریڈیو کی حدود، اور منتخب کردہ سب نیٹ کی اطلاع دیں۔ | نہیں |
| `status` | انجن کے مرحلے، ہاٹ سپاٹ، اور کیا ٹریفک کٹ گئی ہے کی اطلاع دیں۔ | نہیں |
| `start` | سرنگ اور ہاٹ سپاٹ کو اوپر لائیں۔ | ہاں |
| `stop` | انہیں نیچے اتاریں اور ٹیر ڈاؤن جرنل کو دوبارہ چلائیں۔ | ہاں |
| `recover` | رکیں، جرنل کو دوبارہ چلائیں، پھر اسی درخواست سے دوبارہ شروع کریں۔ | ہاں |
| `engine-log` | انجن کی حالیہ لائنوں کو واپس کریں، جو پہلے ہی درست کر دی گئی ہیں۔ | نہیں |
| `cut` | فارورڈ کردہ کلائنٹ ٹریفک کو ڈراپ کریں اور باقی سب کچھ چلتے رہنے دیں۔ | ہاں |
| `restore` | آگے بھیجے گئے کلائنٹ ٹریفک کو واپس رکھیں | ہاں |

ایک درخواست، ایک جواب، ایک رابطہ۔ ایک پیغام 4 بائٹ کا بڑا اینڈین ہوتا ہے۔
لمبائی کے بعد JSON کے اتنے بائٹس۔ لمبائی کے خلاف جانچ پڑتال کی جاتی ہے
`maxFrameBytes` کسی بھی چیز کو مختص یا تجزیہ کرنے سے پہلے، لہذا ایک بڑا پیغام
چار بائٹس اور انکار کی قیمت ہے۔ نامعلوم JSON فیلڈز کے بجائے انکار کر دیا گیا ہے۔
نظر انداز کیا `protocolVersion` ہر درخواست پر چیک کیا جاتا ہے۔ تو ایک سے ایک پینل
کسی دوسرے سے مراعات یافتہ سروس سے بات کرنے پر نام سے انکار ملتا ہے،
ایک فیلڈ کی بجائے خاموشی سے اس کی صفر ویلیو کے طور پر ڈی کوڈ کیا گیا۔

ناکامی کے راستے پر ایک لفظ کے علاوہ کچھ بھی نہیں آتا: ایک `panel.Fault` سے
ایک بند سیٹ، یا دوسرے بند سیٹ سے `privsvc.Refusal`۔ انجن کا اپنا
غلطی کا متن صارف کے کلیدی مواد کو سرایت کرتا ہے، لہذا یہ مراعات یافتہ افراد پر لاگ ان ہوتا ہے۔
طرف اور گرا دیا. جواب کے بارے میں کوئی فیلڈ نہیں ہے جس میں یہ سفر کر سکتا ہے۔

<a id="who-owns-which-package"></a>
### کون سا پیکج کا مالک ہے۔

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

`internal/privsvc` `StartRequest.ConfigJSON` کو `internal/link` کے ساتھ دوبارہ تجزیہ کرتا ہے
پینل پر بھروسہ کرنے کے بجائے یہ کیا ہے۔ یہ انٹرنیٹ بھی چیک کرتا ہے۔
اس مشین کے اپنے طے شدہ راستے، ہاٹ سپاٹ انٹرفیس کے خلاف انٹرفیس
اس مشین کے اپنے `iw list` آؤٹ پٹ کے خلاف، اور چینل کے خلاف کیا
ریڈیو کو قابل استعمال قرار دیا گیا۔

<a id="where-state-lives-and-who-writes-it"></a>
### ریاست کہاں رہتی ہے، اور اسے کون لکھتا ہے۔

دو مصنفین، دو فائلیں، کوئی مشترکہ فائل نہیں۔ کوئی بھی عمل دوسرے کو نہیں لکھتا، تو
اس سے بچانے کے لیے کوئی لاک اور کوئی گمشدہ اپ ڈیٹ نہیں ہے۔ [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md)، "کون
کیا لکھتا ہے"، فیصلے کو ریکارڈ کرتا ہے اور اس سے پہلے کے مسودے کو الٹ دیا جاتا ہے۔

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

مراعات یافتہ فریق کوئی بھی ریاستی فائل نہیں پڑھتا۔ اس کی ضرورت کی ہر چیز پہنچ جاتی ہے۔
شروع کی درخواست. `TestPrivsvcReadsNoStateFile` اس پیکیج کے اپنے ماخذ کو اسکین کرتا ہے۔
اور ناکام ہو جاتا ہے اگر یہ کبھی ایک پڑھتا ہے، جسے کوئی تبصرہ فراہم نہیں کرتا۔

راستوں، طریقوں اور مالکان کی مکمل جدول [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md) میں ہے۔ بندرگاہیں ہیں۔
وہاں بھی فکسڈ: ہاٹ اسپاٹ پر کلائنٹ DNS کے لیے 53، لوپ بیک پر 5354
انجن کا DNS سننے والا، پینل کے لیے 8088، لوپ بیک پر 10808
تشخیصی SOCKS ان باؤنڈ۔

<a id="how-data-flows"></a>
## ڈیٹا کیسے بہتا ہے۔

<a id="a-pasted-share-link-becomes-a-running-tunnel"></a>
### پیسٹ کردہ شیئر لنک ایک چلتی ہوئی سرنگ بن جاتا ہے۔

`startNow` [`internal/panel/handlers.go`](https://github.com/Iman/caspian/blob/main/internal/panel/handlers.go) میں آرڈر کو دستاویز کرتا ہے، اور آرڈر یہ ہے
تین تشکیل کی ناکامیوں کو الگ کیا بتاتا ہے۔ مشین پر کسی چیز کو چھوا نہیں ہے۔
جب تک کہ ریاست 1 اور ریاست 2 دونوں گزر نہ جائیں۔

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

اس ترتیب میں تین تفصیلات لوڈ بیئرنگ ہیں۔

انجن کی دستاویز مختلف وجوہات کی بنا پر دو بار بنائی گئی ہے۔ `internal/link`
آؤٹ باؤنڈ پیدا کرتا ہے اور کچھ نہیں۔ `internal/xcfg` سب کچھ پیدا کرتا ہے۔
اس کے ارد گرد: TUN ان باؤنڈ جس پر کلائنٹ ٹریفک آتا ہے، لوپ بیک SOCKS
تشخیصی اور عبوری macOS سسٹم پراکسی، مقامی DNS کے ذریعے استعمال ہونے والا ان باؤنڈ
سننے والا، حل کرنے والی پالیسی، اور روٹنگ کے اصول۔
اس میں سے کوئی بھی کال کرنے والے کے بھیجے گئے کسی بھی چیز سے نہیں لیا گیا ہے۔

ایک آغاز جو جزوی طور پر ناکام ہو جاتا ہے اسے مکمل طور پر کالعدم کر دیا جاتا ہے۔ جریدہ پہلے ہی
ہر تبدیلی کا الٹا رکھتا ہے، تبدیلی تک پہنچنے سے پہلے ڈسک پر لکھا جاتا ہے۔
دانا ایک ایسا آغاز جو ناکام ہو جاتا ہے مشین کو اسی طرح چھوڑ دیتا ہے جیسے اسے پایا گیا تھا۔

ایک سرور جو جواب نہیں دیتا ہے وہ آدھا لاگو باکس نہیں ہے۔ ہر تبدیلی کامیاب ہوئی،
فائر وال نافذ ہے، اور فارورڈ کلائنٹ ٹریفک بلاک ہے کیونکہ
سرنگ میں کچھ نہیں ہوتا۔ لہذا غلطی کی اطلاع دی جاتی ہے اور کچھ بھی نہیں پھٹا جاتا ہے۔

<a id="the-network-path-of-a-client-packet"></a>
### کلائنٹ پیکٹ کا نیٹ ورک پاتھ

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

لیک بلاک میں صرف ہاٹ اسپاٹ اور اپلنک کا نام ہے۔ یہ کام نہیں روک سکتا
جب سرنگ جاتی ہے، کیونکہ اس میں سرنگ کا ذکر نہیں ہے۔ ہر اصول کہ
اجازت دیتا ہے کلائنٹ ٹریفک سرنگ کا نام رکھتا ہے، لہذا وہ قواعد مماثلت بند کر دیتے ہیں اور
پالیسی سب کچھ چھوڑ دیتی ہے۔

ہر انٹرفیس نام سے مماثل ہوتا ہے اور کبھی بھی انڈیکس سے نہیں۔ ایک انڈیکس حل کیا جاتا ہے جب
رولسیٹ لوڈ ہوتا ہے، لہٰذا سرنگ کو انڈیکس کے لحاظ سے نام دینے والا رولسیٹ لوڈ نہیں ہو سکتا جب کہ
سرنگ نیچے ہے، جو بالکل اسی وقت ہے جب اسے نافذ ہونا ہے۔

پوسٹ روٹنگ کا سلسلہ جان بوجھ کر خالی ہے۔ اپلنک کی طرف ایک بہانا ہے
واحد لائن جو خاموشی سے آلات کو ایک عام راؤٹر میں بدل دے گی۔

<a id="what-the-tunnel-disappearing-does-to-that-path"></a>
### غائب ہونے والی سرنگ اس راستے پر کیا کرتی ہے۔

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

کون سی شاخ ہوتی ہے طے نہیں ہوتی۔ [`internal/netcfg/testdata/PROVENANCE.md`](https://github.com/Iman/caspian/blob/main/internal/netcfg/testdata/PROVENANCE.md)
2026-08-30 کو ہدف سے ایک مشاہدہ ریکارڈ کرتا ہے: `xray0` میں موجود تھا
سروس کے ساتھ نیٹ ورک مینجر کی ڈیوائس کی فہرست بند کر دی گئی، جیسا کہ
`connected (externally)` یہاں کچھ بھی قائم نہیں ہوا کیوں، اور انجن نہیں ہے۔
اس منصوبے کا کوڈ. نہ تو شاخ لیک ہوتی ہے، اور نہ ہی یہ جاننے پر منحصر ہے کہ کون سا
ایک ہوتا ہے. اس لیے بلاک کو صرف ہاٹ اسپاٹ اور کے نام لکھا گیا تھا۔
اپ لنک

<a id="the-dns-path-which-is-not-the-traffic-path"></a>
### DNS پاتھ، جو ٹریفک کا راستہ نہیں ہے۔

یہ وہ حصہ ہے جس سے لوگ غلط ہو جاتے ہیں۔ ایک کلائنٹ کا DNS سوال محض نہیں ہے۔
اجازت دی لیا جاتا ہے۔

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

اس سلسلہ کی چار خصوصیات، ہر ایک اس چیز کے ساتھ جو اسے رکھتی ہے۔

ری ڈائریکٹ منزل کو دوبارہ لکھتا ہے، لہذا ایک آلہ جس میں حل کرنے والا ہارڈ کوڈ ہوتا ہے۔
اس کا جواب یہاں دیا گیا ہے بجائے اس کے کہ اس تک پہنچنے کی اجازت دی جائے جس سے اسے بتایا گیا تھا۔
استعمال کریں منظر نامہ: "ایک کلائنٹ اپنی پسند کے حل کرنے والے تک نہیں پہنچ سکتا"۔

DHCP اس باکس کو ایک بار نام پیش کرتا ہے اور کوئی دوسرا حل کرنے والا نہیں۔ یہ اس کی اپنی قیمت ہے
منظر نامہ کیونکہ اس کا غلط ہونا پوشیدہ ہے: ری ڈائریکٹ دوبارہ لکھے گا۔
ویسے بھی پیکٹ، تو تار پر کچھ بھی غلط نظر نہیں آئے گا۔ منظر نامہ: "باکس
خود کو حل کرنے والے کے طور پر پیش کرتا ہے اور کبھی کسی کا نام نہیں لیتا"۔

`internal/hotspot` کسی بھی dnsmasq اپ اسٹریم سے انکار کرتا ہے جو لوپ بیک ایڈریس نہیں ہے۔
ایک نان لوپ بیک ٹارگٹ ایک سوال ہوگا جو باکس کو ٹنل سے باہر چھوڑتا ہے۔
ہر وہ نام جو ہر کلائنٹ کے لیے پوچھتا ہے۔ انجن کا سننے والا وہی ہے جو وہاں جواب دیتا ہے،
اور `TestLocalDNSDefaultMatchesTheHotspotUpstream` ناکام ہوجاتا ہے اگر دونوں بندرگاہیں بڑھ جاتی ہیں۔
[`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md) اسے کہتے ہیں جوڑ جوڑنا جو خاموشی سے ٹوٹ جاتا ہے: اگر دونوں
بڑھے ہوئے، ہر جوائنڈ ڈیوائس حل ہونا بند کر دیتی ہے جبکہ ہاٹ اسپاٹ اور ٹنل دونوں
صحت مند نظر آتے ہیں.

قاعدہ جو حل کرنے والے کے اپنے سوالات کو سرنگ میں بھیجتا ہے اس کے اوپر بیٹھتا ہے۔
قاعدہ جو نجی پتے براہ راست بھیجتا ہے۔ تو ایک نجی ایڈریس پر حل کرنے والا ہے۔
اب بھی مقامی نیٹ ورک کے بجائے سرنگ کے ذریعے پہنچا۔
`TestLocalDNSQueriesCannotFallOutToTheUplink` اور `TestPrivateRangesRouteDirect`
دو حصوں کو پکڑو.

حل کرنے والا سلسلہ خود تین دائرہ اختیار میں تین آپریٹرز ہے: Quad9's
فلٹرڈ سروس، کلاؤڈ فلیئر فیملی ویرینٹ، اور کلین براؤزنگ سیکیورٹی۔
[`internal/xcfg/resolvers.go`](https://github.com/Iman/caspian/blob/main/internal/xcfg/resolvers.go) ریکارڈ کرتا ہے کہ ہر ایک کیوں، اور جو تقریباً ایک جیسا ہے۔
اسی آپریٹر کا پتہ یہ جان بوجھ کر نہیں ہے۔ کوئی گوگل حل کرنے والا ظاہر نہیں ہوتا ہے۔
کسی بھی ڈیفالٹ میں، اور `TestNoGoogleAnywhereInGeneratedConfigs` ہر اسکین کرتا ہے۔
ایک کے لیے تیار کردہ دستاویز۔

دوسری بندرگاہوں کو سنبھالا جاتا ہے اور ان میں سے ایک نہیں ہوسکتی ہے:

```mermaid
flowchart LR
    DOT["DNS over TLS<br/>tcp 853"] --> REJ["reject with tcp reset,<br/>so the device falls back to port 53"]
    DOQ["DNS over QUIC<br/>udp 853"] --> DRP["drop"]
    DOH["DNS over HTTPS<br/>port 443"] --> CAR["carried through the tunnel like any HTTPS.<br/>Not a leak. Not visible to anything here."]
```



<!-- Caspian guide navigation -->

Caspian گائیڈز: [سیٹ اپ اور معاون پروٹوکول](https://github.com/Iman/caspian/wiki/Home.ur) · [ڈی پی آئی کو روکنے کے لیے SNI کی جعل سازی: سیٹ اپ اور حدود](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ur)۔

</div>


<!-- English-source-sha256: 07a2e0584db74a162eb7938428d733054b00788748889213f96e0e0679d0f650 -->
