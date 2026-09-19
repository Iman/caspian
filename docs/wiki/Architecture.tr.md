<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Architecture) | [فارسی](https://github.com/Iman/caspian/wiki/Architecture.fa) | [Русский](https://github.com/Iman/caspian/wiki/Architecture.ru) | [中文](https://github.com/Iman/caspian/wiki/Architecture.zh) | [العربية](https://github.com/Iman/caspian/wiki/Architecture.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Architecture.tr) | [اردو](https://github.com/Iman/caspian/wiki/Architecture.ur)

</div>

<a id="architecture-and-data-flow"></a>
# Mimari ve veri akışı



[Caspian wiki'si](https://github.com/Iman/caspian/wiki/Home.tr)

> Bu kılavuz mevcut README'den alınmıştır. Ölçümleri orijinal tarihlerini koruyor; bu belgeleme hamlesi yeni bir test çalıştırmasını bildirmez.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="architecture"></a>
## Mimarlık

<a id="two-processes-one-binary"></a>
### İki süreç, bir ikili

Bir ikili, alt komut tarafından seçilen iki rolde çalışır. Bölünme öyle bir şekilde var ki
Kullanıcı girişini ayrıştıran ve HTTP'ye hizmet eden kısımdaki hata,
kökü tutan kısım. [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md), "İki süreç, bir ikili"
bunun sabit beyanı.

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

[`cmd/caspian/main.go`](https://github.com/Iman/caspian/blob/main/cmd/caspian/main.go) iki rolü kendi kullanım metninde yazdırır:

caspian service --ayrıcalıklı kök: yollar, güvenlik duvarı, erişim noktası, motor
caspian service --panel caspian kullanıcısını: web paneli, ayrıcalıklı bir şey yok

<a id="the-socket-and-why-the-vocabulary-is-closed"></a>
### Soket ve kelime dağarcığının neden kapalı olduğu

[`internal/panel/priv.go`](https://github.com/Iman/caspian/blob/main/internal/panel/priv.go), tüm bölünmenin var olduğu kuralı belirtir: "A
istemcisinden bir yol ve argüman listesi alan ayrıcalıklı bir yardımcı değildir.
bir sınır; herhangi bir şeyi root olarak çalıştırmanın bir yoludur." Noktalı virgül onlarındır.
cümle aynen alıntılanmıştır, çünkü bir kuralın başka sözcüklerle ifade edilmesi kural değildir.

Dolayısıyla panel "bunu çalıştır" ifadesini ifade edemez. Sekiz eylemden yalnızca birini adlandırabilir,
ve her birinin ne anlama geldiğine ayrıcalıklı taraf karar verir. `panel.Actions` bu
kapalı küme ve bir yöntem varsa `TestActionVocabularyMatchesTheInterface` başarısız olur
listede isim olmadan arayüze eklendi.

| Eylem | Ayrıcalıklı taraf ne yapar? | Makineyi değiştirir |
|---|---|---|
| `detect` | Arayüzleri, radyonun sınırlarını ve seçilen alt ağı rapor edin | hayır |
| `status` | Motor aşamasını, sıcak noktayı ve trafiğin kesilip kesilmediğini bildirin | hayır |
| `start` | Tüneli ve sıcak noktayı yukarı getirin | evet |
| `stop` | Onları aşağı indirin ve sökme günlüğünü tekrar oynatın | evet |
| `recover` | Durdurun, günlüğü yeniden oynatın ve ardından aynı istekten yeniden başlayın | evet |
| `engine-log` | Motorun zaten düzenlenmiş olan son satırlarını döndür | hayır |
| `cut` | İletilen istemci trafiğini bırakın ve diğer her şeyi çalışır durumda bırakın | evet |
| `restore` | İletilen istemci trafiğini geri koyun | evet |

Tek istek, tek yanıt, tek bağlantı. Bir mesaj 4 baytlık big-endian'dır
uzunluk ve ardından bu kadar JSON baytı gelir. Uzunluk kontrol edilir
`maxFrameBytes` herhangi bir şey tahsis edilmeden veya ayrıştırılmadan önce, dolayısıyla büyük boyutlu bir mesaj
dört bayta mal olur ve bir ret. Bilinmeyen JSON alanları reddedilmek yerine reddedilir
görmezden gelindi. `protocolVersion` her istekte kontrol edilir. Yani birinden bir panel
başka birinden ayrıcalıklı bir hizmetle konuşmayı serbest bırakmanın ismen reddedilmesi,
sessizce sıfır değeri olarak kodu çözülen bir alan yerine.

Başarısızlık yolunda tek bir kelime dışında hiçbir şey geri dönemez: `panel.Fault`
kapalı bir küme veya ikinci bir kapalı kümeden bir `privsvc.Refusal`. Motor kendi
hata metni kullanıcının anahtar materyalini gömer, böylece ayrıcalıklı oturum açılır
yan ve düştü. Yanıt üzerinde seyahat edebileceği hiçbir alan yok.

<a id="who-owns-which-package"></a>
### Kim hangi paketin sahibi

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

`internal/privsvc`, `StartRequest.ConfigJSON`'yi `internal/link` ile yeniden ayrıştırır
Panelin bunu yaptığına güvenmek yerine. Ayrıca interneti de kontrol ediyor
Bu makinenin kendi varsayılan rotasına karşı arayüz, sıcak nokta arayüzü
bu makinenin kendi `iw list` çıkışına ve kanala karşı
radyonun kullanılabilir olduğu bildirildi.

<a id="where-state-lives-and-who-writes-it"></a>
### Devlet nerede yaşıyor ve onu kim yazıyor?

İki yazar, iki dosya, paylaşılan dosya yok. Hiçbir süreç diğerininkini yazmaz, dolayısıyla
Korunacak bir kilit veya kayıp güncelleme yoktur. [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md), "Kim
ne yazıyor", kararı ve tersine çevirdiği önceki taslağı kaydeder.

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

Ayrıcalıklı taraf hiçbir durum dosyasını okumaz. İhtiyacı olan her şey geliyor
başlatma isteği. `TestPrivsvcReadsNoStateFile` bu paketin kendi kaynağını tarar
ve bir yorumun sağlayamayacağı bir tanesini okursa başarısız olur.

Yolların, modların ve sahiplerin tam tablosu [`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md)'dedir. Bağlantı noktaları
burada da düzeltildi: etkin noktadaki istemci DNS'si için 53, geri döngüde 5354
motorun DNS dinleyicisi, panel için 8088, geri döngüde 10808
tanılama ÇORAPLARI geliyor.

<a id="how-data-flows"></a>
## Veri akışı nasıl

<a id="a-pasted-share-link-becomes-a-running-tunnel"></a>
### Yapıştırılan bir paylaşım bağlantısı çalışan bir tünele dönüşür

[`internal/panel/handlers.go`](https://github.com/Iman/caspian/blob/main/internal/panel/handlers.go)'deki `startNow` siparişi belgeliyor ve sipariş şu şekilde:
üç yapılandırma hatasını birbirinden ayıran şey nedir? Makinedeki hiçbir şeye dokunulmuyor
durum 1 ve durum 2'nin her ikisi de geçene kadar.

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

Bu dizideki üç detay yük taşıyor.

Motor belgesi farklı nedenlerle iki kez oluşturulmuştur. `internal/link`
gideni üretir ve başka hiçbir şey yapmaz. `internal/xcfg` her şeyi üretir
etrafında: istemci trafiğinin geldiği TUN girişi, geri döngü SOCKS
Tanılama tarafından kullanılan gelen ve geçici macOS sistem proxy'si olan yerel DNS
dinleyici, çözümleyici politikası ve yönlendirme kuralları.
Bunların hiçbiri arayanın gönderdiği herhangi bir şeyden alınmaz.

Kısmen başarısız olan bir başlangıç tamamen geri alınır. Dergi zaten
Değişiklik istenen noktaya ulaşmadan önce diske yazılan her değişikliğin tersini tutar.
çekirdek. Başarısız olan bir başlatma, makineyi bulunduğu şekilde bırakır.

Yanıt vermeyen bir sunucu yarı uygulanmış bir kutu değildir. Her değişiklik başarılı oldu
güvenlik duvarı yürürlükte ve iletilen istemci trafiği engellendi çünkü
tünel hiçbir şey taşımaz. Böylece arıza bildirilir ve hiçbir şey yıkılmaz.

<a id="the-network-path-of-a-client-packet"></a>
### Bir istemci paketinin ağ yolu

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

Sızıntı bloğu yalnızca sıcak noktayı ve yukarı bağlantıyı adlandırır. Çalışmayı durduramaz
Tünel gittiğinde, çünkü tünelden bahsetmiyor. Her kural
istemci trafiğinin tüneli adlandırmasına izin verir, böylece bu kurallar eşleşmeyi durdurur ve
politika her şeyi bırakır.

Her arayüz ada göre eşleştirilir, asla dizine göre eşleştirilmez. Bir dizin şu durumlarda çözümlenir:
kural kümesi yüklenir, dolayısıyla tüneli dizine göre adlandıran bir kural kümesi,
Tünel kapandı, tam da yürürlüğe girmesi gereken zamanda.

Postrouting zinciri bilerek boş bırakılmıştır. Yukarı bağlantıya yönelik bir maskeli balo
cihazı sessizce sıradan bir yönlendiriciye dönüştürecek tek hat.

<a id="what-the-tunnel-disappearing-does-to-that-path"></a>
### Tünelin kaybolması o yola ne yapıyor?

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

Hangi şubenin olacağı henüz belirlenmedi. [`internal/netcfg/testdata/PROVENANCE.md`](https://github.com/Iman/caspian/blob/main/internal/netcfg/testdata/PROVENANCE.md)
30-08-2026 tarihinde hedeften bir gözlem kaydediyor: `xray0` şurada mevcuttu
NetworkManager'ın hizmetin kapalı olduğu cihaz listesi,
`connected (externally)`. Burada bunun nedenini açıklayan hiçbir şey yok ve motor da
bu projenin kodu. Hiçbir dal sızıntı yapmaz ve hangisinin hangisi olduğunu bilmeye bağlı değildir.
biri olur. Bu nedenle blok yalnızca etkin noktayı ve etkin noktayı adlandıracak şekilde yazılmıştır.
yukarı bağlantı.

<a id="the-dns-path-which-is-not-the-traffic-path"></a>
### Trafik yolu olmayan DNS yolu

İnsanların yanlış anladığı kısım burası. Bir müşterinin DNS sorusu yalnızca
izin verildi. Alındı.

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

Bu zincirin dört özelliği, her biri onu tutan şeyle birlikte.

Yönlendirme hedefi yeniden yazar, böylece kodlanmış çözümleyiciye sahip bir cihaz
kendisine söylenen kişiye ulaşması yerine burada yanıtlanır
kullanın. Senaryo: "istemci kendi seçtiği çözümleyiciye ulaşamıyor".

DHCP teklifi bu kutuyu bir kez adlandırır ve başka çözümleyici içermez. Bu kendine değer
senaryo çünkü yanlış yapmak görünmez: yönlendirme yeniden yazar
paketler zaten, böylece kablodaki hiçbir şey yanlış görünmez. Senaryo: "kutu
kendisini çözümleyici olarak sunar ve asla başkasının adını vermez".

`internal/hotspot`, geridöngü adresi olmayan herhangi bir dnsmasq yukarı akışını reddeder.
Geridöngü olmayan hedef, kutuyu tünelin dışında bırakan bir sorgu olacaktır.
her müşterinin istediği her isim. Motorun dinleyicisi orada cevap veriyor,
ve iki bağlantı noktası sürüklenirse `TestLocalDNSDefaultMatchesTheHotspotUpstream` başarısız olur.
[`docs/LAYOUT.md`](https://github.com/Iman/caspian/blob/main/docs/LAYOUT.md) buna sessizce kırılan eşleşmeyi çağırıyor: eğer ikisi
sürüklenme, sıcak nokta ve tünelin her ikisi de çalışırken, birleştirilen her cihaz çözümlemeyi durdurur
sağlıklı görünün.

Çözümleyicinin kendi sorgularını tünele gönderen kural,
özel adresleri doğrudan gönderen kural. Yani özel bir adresteki çözümleyici
hâlâ yerel ağ yerine tünel yoluyla ulaşılıyor.
`TestLocalDNSQueriesCannotFallOutToTheUplink` ve `TestPrivateRangesRouteDirect`
iki yarımı tutun.

Çözümleyici zincirinin kendisi üç yargı bölgesindeki üç operatörden oluşur: Quad9'lar
filtrelenmiş hizmet, Cloudflare AİLE çeşidi ve CleanBrowsing Güvenliği.
[`internal/xcfg/resolvers.go`](https://github.com/Iman/caspian/blob/main/internal/xcfg/resolvers.go) her birinin nedenini ve hangilerinin neredeyse aynı olduğunu kaydeder
Aynı operatörün adresi kasıtlı olarak verilmemiştir. Hiçbir Google çözümleyici görünmüyor
herhangi bir varsayılanda ve `TestNoGoogleAnywhereInGeneratedConfigs` her
biri için belge oluşturuldu.

Diğer bağlantı noktaları işlenir ve bunlardan biri olamaz:

```mermaid
flowchart LR
    DOT["DNS over TLS<br/>tcp 853"] --> REJ["reject with tcp reset,<br/>so the device falls back to port 53"]
    DOQ["DNS over QUIC<br/>udp 853"] --> DRP["drop"]
    DOH["DNS over HTTPS<br/>port 443"] --> CAR["carried through the tunnel like any HTTPS.<br/>Not a leak. Not visible to anything here."]
```



<!-- Caspian guide navigation -->

Caspian kılavuzları: [kurulum ve desteklenen protokoller](https://github.com/Iman/caspian/wiki/Home.tr) · [DPI'yı aşmak için SNI sahtekarlığı: kurulum ve sınırlar](https://github.com/Iman/caspian/wiki/SNI-Spoofing.tr).


<!-- English-source-sha256: 07a2e0584db74a162eb7938428d733054b00788748889213f96e0e0679d0f650 -->
