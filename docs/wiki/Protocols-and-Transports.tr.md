<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Protocols-and-Transports) | [فارسی](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa) | [Русский](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru) | [中文](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh) | [العربية](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.tr) | [اردو](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ur)

</div>

<a id="protocols-and-transports"></a>
# Protokoller ve aktarımlar



[Caspian wiki'si](https://github.com/Iman/caspian/wiki/Home.tr)

> Bu kılavuz mevcut README'den alınmıştır. Ölçümleri orijinal tarihlerini koruyor; bu belgeleme hamlesi yeni bir test çalıştırmasını bildirmez.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="what-you-can-paste-and-what-it-will-refuse"></a>
## Neyi yapıştırabilirsiniz ve neyi reddedecek

Yapılandırmayı getiriyorsunuz. Bu, kutunun kabul ettiği koddan alınan şeydir.
bir istek listesinden ziyade kabul etmeyi yapar. Her sıra ölçüldü
`internal/link` ve sabitlenmiş motor.

|  | Çalışıyor | Reddedildi |
|---|---|---|
| Bağlantıları paylaş | `vless://` `vmess://` `ss://` `socks://` `trojan://` `hysteria2://` `hy2://` | `tuic://` `ssr://` `wireguard://` `anytls://` `naive+https://` `hysteria://` (versiyon 1) |
| Yapıştırılan belgeler | Clash ve Clash.Meta YAML, ham xray JSON, her satıra bir bağlantı listesi, bir base64 abonelik blobu | bir abonelik URL'si, base64 ile sarılmış bir Clash belgesi, bir JSON dizisi, ilk satırı yorum olan metin |
| Taşımalar | `raw` (aynı zamanda `tcp` olarak da yazılır), `ws`, `grpc`, `httpupgrade`, `xhttp` (ayrıca `splithttp`), `kcp` ve `mkcp` | `h2`, `h3`, `http`, `quic`, `gun` |
| Güvenlik | `none`, `tls`, `reality` | `xtls` (eski tür), `allowInsecure` |
| VLESS akışı | `xtls-rprx-vision`, `xtls-rprx-vision-udp443` veya hiçbiri | diğer tüm değerler |

Reddedilen sütundaki `h2` ve `h3` taşıma İSİMLERİ'dir. HTTP/2 ve HTTP/3
kendileri taşınır: `type=xhttp` ile `security=tls` ve TLS ALPN
hangisi olduğuna karar verir. Bkz. [HTTP/2 ve HTTP/3 farklı bir şekilde taşınır
ad](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.tr#http2-and-http3-are-carried-under-a-different-name).

Altı şey insanları şaşırttığı için dipnot yerine burada yer alıyorlar:

Yalnızca İLK bağlantı kullanılır. Kırk sunucuyu yapıştırın ve birini yapılandırın; the
panel size kaç tane bulduğunu söyler. `ss://` ve `socks://`'nin base64 formuna ihtiyacı var
Kullanıcı bilgilerinin ve düz `method:password@host` yazımı
reddetti. REALITY yalnızca `raw`, `xhttp` ve `grpc` üzerinde çalışır;
WebSocket, daha sonra başarısız olmak yerine, yapıştırma zamanında motor tarafından reddedilir.
Motorun kendisi küçük olmasa da `security=` burada küçük harf olmalıdır.
ve büyük harf `TLS` size `none` olarak bildirilir. Bir `plugin=`
`ss://` bağlantısındaki parametre, söylenmeden göz ardı edilir. Ve bir abonelik
Yapılandırma kutusuna yapıştırılan URL reddedildi çünkü bu kutu yapılandırmaları alıyor:
adresi yanındaki abonelik alanına gider ve Caspian yalnızca onu getirir
düğmeye bastığınızda tünelden.

Bunlardan hangisinin gerçek bayt taşıdığı ve hangisinin olduğu da dahil olmak üzere resmin tamamı
çıkış adresinin yakalandığı donanımda uçtan uca kanıtlanmıştır.
[Protokoller ve aktarımlar](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.tr#protocols-and-transports). Bunlar üç farklı
iddiaları var ve bu proje bunların bulanıklaşmasına izin vermiyor.

<a id="protocols-and-transports-1"></a>
## Protokoller ve aktarımlar

Bir paylaşım bağlantısı üç ayrı şeyi taşır ve onları ayrı tutmaya yardımcı olur:
proxy protokolü, onu taşıyan aktarım ve sarılmış şifreleme katmanı
bu ulaşımın etrafında. TLS ve VLESS bağlantısıyla WebSocket üzerinden bir VLESS bağlantısı
REALITY ile düz TCP üzerinden aynı türe ulaşan aynı protokoldür
sunucuya iki farklı yoldan. Farklı şekillerde başarısız olurlar.

Proxy protokolleri yukarıda listelenen yedi şemadır. VLESS en çok
bu belgedeki örneklerde kullanılmaktadır çünkü REALITY bunun için tasarlanmıştır. Hiçbir şey
cihazda buna özeldir. Ayrıştırıcı bir açıklama üretir,
`internal/xcfg`, bunun ve kutunun geri kalanının etrafında bir motor belgesi oluşturur
hangi protokolü taşıdığını bilmiyor.

Aktarımlar xray-core'dan gelir ve satıcı ayrıştırıcıda adlandırılır:

- `tcp`, `raw` olarak da yazılır
- `ws`, WebSocket için
- `httpupgrade`
- `xhttp`, daha önce SplitHTTP olarak adlandırılan protokol. Her iki yazım da ayrıştırılır
- `grpc`
- mKCP için `kcp` ve `mkcp`

`h2`, `http`, `h3` ve `quic` bu listede yok. Bu pinin motor versiyonu
bunları kaldırdık, dolayısıyla bağlantı isteyen bir bağlantı taşınmak yerine reddedildi ve
`internal/link`'deki `TestRemovedTransportsAreRefusedWithASentence` bunu tutar
yerinde reddetme.

Reddetmenin ne kadar iyi okunacağı, içerideki rotaya bağlıdır. Bunlardan birinin adını veren bir Clash belgesi
taşımayla ilgili ceza alır. `type=` parametresinde aynı aktarım
bir paylaşım bağlantısında genel olarak "yapıştırılan metindeki hiçbir şey proxy değildi
bağlantı bu kutunun anladığıdır", bu doğru ve yararsızdır.
`TestRemovedTransportInAURIIsReportedLessWell` bu farkı pinliyor, bu yüzden
sürprizden ziyade bilinen boşluk.

<a id="http2-and-http3-are-carried-under-a-different-name"></a>
### HTTP/2 ve HTTP/3 farklı bir ad altında taşınır

`type=h2` veya `type=quic`'nin reddedilmesi, kutunun bunları konuşamayacağı anlamına gelmez.
Bu, yazının taşındığı anlamına gelir. XHTTP her ikisinin de yerini aldı ve kendi HTTP'sini seçti.
aktarım adı yerine TLS ALPN'den alınan sürüm:

| ne istiyorsun | Ne yazmalı |
|---|---|
| QUIC olan HTTP/3 | `type=xhttp`, `security=tls`, `alpn=h3` ve `mode=stream-one` ile |
| HTTP/2 | `type=xhttp` ile `security=tls` ve tam olarak `h3` olmayan herhangi bir ALPN |
| QUIC, XHTTP'siz | altında QUIC olan ve `alpn=h3`'ye ihtiyaç duyan bir `hysteria2://` bağlantısı |

Anahtarlar motora dokunulmadan ulaşıyor: `internal/xcfg` giden sinyali şu şekilde taşıyor:
opak JSON ve hiçbir zaman kodunu çözmez, bu nedenle `alpn`, `mode`, `xmux` ve QUIC ayarlama
blok tam olarak yapıştırıldığı gibi gelir.

H3'ü mü alacağınızı yoksa sessizce başka bir şeyi mi alacağınızı dört ayrıntı belirler:

`alpn` tam olarak bir değer olmalı ve bu değer `h3` olmalıdır. Yazma
`alpn=h3,h2`, motor bir liste aldığından size hiçbir uyarı vermeden HTTP/2 verir
sürüm 2 için istek olarak başka herhangi bir uzunlukta. REALITY, HTTP/2'yi her zaman zorlar
mevcut olduğundan REALITY ve h3 birbirini dışlar ve onları eşleştirmek
bir hata yerine h2'siniz. `mode` açıkça ayarlanmalıdır çünkü
varsayılan olarak motor şekli `stream-one` yerine `packet-up` olarak çözümlenir
QUIC değişimi olarak adlar. Ve bölünmüş yükleme için `downloadSettings` ve
indirme `mode: stream-one` ile birlikte reddedildi; bu kombinasyonun ihtiyacı var
`stream-up`.

Kelime dağarcığında bir çarpışma açıkça belirtilmeye değer çünkü bir kelime gibi okunuyor
çelişki: `type=h3` reddedildi ve `alpn=h3` gerekli. Onlar
farklı alanlar. İlki artık var olmayan bir taşımanın adını verir; ikinci
TLS içinde müzakere edilen protokolü adlandırır.

Bu konfigürasyonlar kutu tarafından kabul edilir ve doğrulanır. Henüz yapmadılar
buradan canlı bir sunucuya yönlendirildiniz, bu nedenle satırı motorun satırı olarak değerlendirin
Bu projenin çalışmayı izlediği bir şey olmaktan ziyade yetenek.

Güvenlik katmanı `reality`, `tls` veya `none`'dir.

Her kombinasyon eşit derecede yararlı değildir. REALITY normalde düz renkle eşleştirilir
TCP, çünkü tüm yöntemi gerçek bir sitenin TLS anlaşmasını ödünç almaktır.
onu başka bir TLS katmanına sarmak asıl amacı boşa çıkarır. WebSocket, HTTP Yükseltme ve
XHTTP, denetleyen bir şeye sıradan web trafiği gibi görünmek için mevcuttur.
bağlantı kurarlar ve genellikle sıradan bir bağlantıyla aynı nedenden dolayı TLS ile eşleştirilirler.
web sitesi. `security=none`'ye sahip WebSocket, iki kez düşünülmesi gereken tek şekildir
hakkında. Telgraftaki düz metindir ve yalnızca başka bir şey olduğunda anlamlıdır
TLS'yi sonlandıran bir CDN gibi şifrelemeyi zaten sağlıyor
sunucu.

<a id="three-different-claims-kept-apart"></a>
### Üç farklı iddia birbirinden ayrı tutuldu

Aşağıdaki ayrım bu belgedeki en önemli şeydir. Okuyun
satırlardan önce sütun başlıkları.

| Talep | Neye dayanıyor | Değeri nedir? |
|---|---|---|
| Ayrıştırıcı bunu kabul eder | `internal/link` ve kararlı bir altın motor belgesi | Belge stabildir. Hiçbir şey çevrilmedi |
| Bayt taşır | `test/tunnel`, geri döngüde gerçek bir xray çekirdekli sunucu | Trafik protokol üzerinden taşındı. Çıkış IP'si yok, cihaz yok, internet yok |
| Baştan sona kanıtlandı | `test/hardware`, erişim noktasında gerçek bir telefon | Gerçek trafik kutudan ayrıldı ve çıkış adresi yakalanıp adlandırıldı |

<a id="what-has-carried-bytes-through-a-real-server"></a>
### Gerçek bir sunucu üzerinden baytları ne taşıdı?

`test/tunnel` tarafından eklendi. Ayrıştırıcının kabul ettiği her şema uçtan uca sürülür
Bu modülün kendi bağımlılığından oluşturulmuş gerçek bir xray-core örneğine karşı ve
`internal/engine`'nin kullandığı yükleyiciyle yüklendi. Müşteri tarafı ise
ürün yolu, değiştirilmemiş: `link.Parse`, ardından `xcfg.Build`, ardından
`engine.Engine.Start`. Hiçbir yapılandırma elle yazılmaz.

| protokol | ulaşım | güvenlik | bir HTTP isteği taşır |
|---|---|---|---|
| VLESS | TCP (ham) | hiçbiri | evet |
| VMess | TCP (ham) | hiçbiri | evet |
| Shadowsocks, aes-256-gcm | TCP (ham) | hiçbiri | evet |
| ÇORAP | TCP (ham) | hiçbiri | evet |
| Trojan | TCP (ham) | TLS, özet tarafından sabitlendi | evet |
| Hysteria2 ve `hy2` takma adı | HIZLI | TLS, özet tarafından sabitlendi | evet |

Dört kontrol, tüneli atlayan bir isteğin geçmesini durdurur ve dördü de
düzyazıda iddia edilmek yerine koşun. Müşteriye asla nerede olduğu söylenmez.
Menşei `.invalid` adı ve tuzağın bağlantı noktasıdır ve ona verilir. isim
çözülemiyor ve makinede bir çözümleyici varsa paket bunu yüksek sesle söylüyor
yine de cevaplıyor. Kaynak, yalnızca isteğin nereye gönderildiğini değil, aynı zamanda isteğin nereye yönlendirildiğini de kontrol eder.
geldi. Tuzak kendi isabetlerini sayar ve tünellenmiş bir isteğin eklenmesi gerekir
hiçbiri. `TestEveryCarriageProofCanFail` ve
`TestTheProofRejectsARequestThatDidNotGoThroughTheTunnel` bunları yapan şeydir
niyetten ziyade kanıtları kontrol eder.

Her satırı dar bir şekilde okuyun. Hysteria2 dışındaki her satır ham TCP üzerinden çalışır. Satır sürücüsü yok
Sunucu tarafının gerçek bir el sıkışma hedefine ihtiyacı olan REALITY. Shadowsocks:
yalnızca aes-256-gcm, çünkü 2022 şifreleri farklı bir kod yolu kullanıyor. Her satır
bir TCP isteği taşır ve UDP ilişkilendirmesi kapalıdır. Her şey geri döngüde, yani
hiçbir çıkış IP'si yakalanmaz ve hiçbiri yakalanamaz.

`TestEveryProtocolTheParserAcceptsIsDrivenEndToEnd` kabul edilen şemayı okur
`internal/link`'nin kaynağından liste çıkar, dolayısıyla sekizinci şema eklenemez
burada satır olmadan.

<a id="what-has-actually-been-proven-on-hardware"></a>
### Donanımda gerçekte kanıtlanmış olan şey

Aşağıdaki tablo, yakalanan bir çıkış IP'si ile gerçek trafiğin geçtiği yerleri göstermektedir. o
ayrıştırıcının kabul ettiği şey değildir ve geri döngü paketinin taşıdığı şey değildir.

| protokol | ulaşım | güvenlik | baştan sona kanıtlanmış |
|---|---|---|---|
| VLESS | TCP (ham) | REALITY | evet, üç ayrı sunucuda |
| VLESS | ws (WebSocket) | hiçbiri artı VLESS Şifreleme | evet |
| VLESS | ws (WebSocket) | TLS | evet, CDN aracılığıyla |
| VLESS | httpyükseltme | TLS | evet, CDN aracılığıyla |
| VLESS | xhttp | TLS | evet |
| VMess, Trojan, Shadowsocks, ÇORAP, Hysteria2 | herhangi biri | herhangi biri | hayır |

Bunların her biri, sisteme bağlı gerçek bir telefonda gerçek bir tarayıcı çalıştırılarak kanıtlanmıştır.
sıcak nokta. Çıkış adresi iki bağımsız kaynaktan alındı ve eşleştirildi
yapılandırma adlarını sunucuya aktarın. Üç farklı sunucu kullanıldı ve
her biri farklı bir adres döndürdü, bu nedenle tekrarlanan veya önbelleğe alınan bir okuma yapılamaz
çalışan bir tünelle karıştırıldı.

Kanıtlanmayan bir sıra bozulduğu iddiası değildir. Bu bir iddiadır ki
kimse uzak uçtan gelen bir paketi izlemedi, bu farklı bir şey
ve bu projenin kanıt olarak kabul ettiği tek şey. Motor belgelerinin her biri
aktarım IS'yi altın bir dosya olarak sabitlenmiş olarak üretir, dolayısıyla nasıl bir değişiklik yapılacağı
oluşan bir fark olarak görünür. Bu, belgenin sağlam olduğunu ve hiçbir şey söylemediğini kanıtlar
ulaşımın bağlanıp bağlanmayacağı hakkında.

<a id="why-a-row-with-no-transport-security-is-still-encrypted"></a>
### Aktarım güvenliği olmayan bir satır neden hâlâ şifreleniyor?

Yukarıdaki `security` sütunu, aktarımın ETRAFINA SARILI katmanla ilgilidir ve
`none` "şifreleme yok" anlamına gelmez. TLS olmadığı ve REALITY olmadığı anlamına gelir. O
Bu konuda kesin olmaya değer çünkü diğer şekilde okumak endişe verici olabilir
ve onu çok cömertçe okumak daha kötü olurdu.

VLESS tek başına şifreleme taşımaz. Bu, vatansız bir protokoldür ve
gizliliği sağlamak için altındaki katman, normalde REALITY veya
TLS. `security=none` ile WebSocket üzerinden bir VLESS bağlantısı ve başka hiçbir şey OLMAYACAKTIR
kablodaki düz metin ve her pakette çıkış adresi kanıtlanacak
yoldaki herhangi bir şey tarafından okunabiliyordu.

Bu satırı güvenli kılan, bağlantının içinde taşınan VLESS Şifrelemesidir.
`encryption=` parametresi. ML-KEM-768 adlı hibrit bir anahtar değişimidir.
VLESS katmanının kendisine uygulanan X25519 ile birleştirilmiş kuantum sonrası direnç
onun altında değil. Yani trafik şifrelenir ve şu şekilde şifrelenir:
Bugün onu kaydeden bir saldırgana karşı güvende kalmak için tasarlanmış bir şey ve
daha sonra bir kuantum bilgisayarı var. `encryption=none` VE'yi taşıyan bir bağlantı
`security=none`'de bunların ikisi de yok ve bu reddedilecek kombinasyon.

Bu, Gürültü Protokolü Çerçevesi DEĞİLDİR (noiseprotocol.org). Bunda hiçbir şey yok
cihazda, satıcının paylaşım bağlantısı ayrıştırıcısında veya motorda Gürültü uygular.
"Gürültü" kelimesi xray-core'un konfigürasyonunda alakasız bir şey için görünüyor,
Tel üzerindeki şeklini değiştirmek için trafiği rastgele baytlarla doldurmak
el sıkışma değil, kafa karıştırma. Bu satıra özelliğini veren şey
gizlilik VLESS Şifrelemedir ve adı önemlidir çünkü ikisi
farklı garantiler sağlar.

30.08.2026 tarihinde varsayılmak yerine ÖLÇÜLDÜ. Bu paket,
Alan alan giden. Ayrıştırıcının ürettiğini yeniden serileştirir ve
protokol ayarları opak bir damla gibi ilerler. Bu yüzden parametre
hayatta kalır. Hayatta kalmayı bırakırsa hiçbir şeyin kırılmamasının nedeni de budur: alan yok
eksik olacak, hiçbir tür değişmeyecek ve başka hiçbir test bunu fark etmeyecektir.
Tünel, tüm kontroller hala yeşilken, bir kullanıcının trafiğini açıkta taşıyordu.
`internal/link`'deki `TestVLESSEncryptionSurvivesIntoTheEngineDocument`,
koruma ve daha önce tam olarak bu sessiz düşüşe karşı başarısız olduğu izlendi
tutuldu.

<a id="a-certificate-name-that-did-not-match-and-the-client-side-fix"></a>
### Eşleşmeyen bir sertifika adı ve istemci tarafı düzeltmesi

Bir sonuç kaydedilmeye değer çünkü bu cihazın doğru bir şekilde arızalanmasıdır.
kağıt üzerinde çalışmayı reddediyor. Sunucunun kendi adresine işaret eden iki yapılandırma
önünde CDN'nin TLS adını taşırken. Motor şunları bildirdi:

taşıma/internet/httpupgrade: istek çevrilemedi...
tls: sertifika doğrulanamadı: x509: sertifika şunun için geçerli:
      <the apex>, not <the cdn subdomain>

Bu, istenen adla gerçekten eşleşmeyen bir sertifikadır ve
bunu reddetmek istediğiniz davranıştır. Bunu kabul etmek, tünelin
herhangi bir sertifikaya sahip herhangi bir şey tarafından sonlandırılabilir.

Hem nedeni hem de çözümü istemci tarafındadır ve sunucuda herhangi bir değişiklik yapılmaz.
ihtiyaç vardı. Bir paylaşım bağlantısı, insanların eşleşmesi ve eşleştirmesi gerektiğini düşündüğü iki adı taşır
değil:

sni TLS adı sertifikayı doğrular
sunucunun isteği yönlendirdiği adı barındırır, bir HTTP başlığı

Arızalı bağlantılar HER İKİSİNDE de CDN'nin adını taşıyordu. Çalışan CDN aracılığıyla,
çünkü CDN bunun için bir sertifikaya sahiptir. Doğrudan orijine işaret etti
olamaz, çünkü kaynak yalnızca tepe noktası için bir sertifikaya sahiptir. `sni`'yi şu şekilde ayarlayın:
sertifikanın gerçekte taşıdığı adı girin ve adı `host` olarak bırakın.
sunucu rotaları:

sni=example.com ana bilgisayar=cdn.example.com

30.08.2026'DA ÖLÇÜLDÜ. Sertifika hatası nedeniyle başarısız olan iki bağlantı
yukarıda her ikisi de bu tek değişiklikten sonra birbirine bağlandı. Çıkış adresleri şuradan ele geçirildi:
iki bağımsız kaynak ve kendi sunucularıyla eşleştirilmiş, DNS sızıntısı ve
başarısızlıkla kapatılan kontroller aynı çalıştırmada geçildi.

Dolayısıyla, bir aktarım yalnızca doğrudan orijine yönlendirildiğinde başarısız olursa, `sni` ile karşılaştırın
şüphelenmeden önce menşe belgesinin konu alternatif adlarına karşı
taşıma. `openssl s_client -connect <address>:443 -servername <name>`
sunucunun gerçekte ne sunduğunu yazdırır.

<a id="the-panel-takes-a-pasted-link-and-not-an-image"></a>
### Panel, bir resmi değil, yapıştırılan bir bağlantıyı alır

Bir QR görüntüsünün bırakılması tasarım bölümü 5.2'de açıklanmaktadır ve **değildir.
uygulandı**. `internal/panel/qr` yalnızca bir kodlayıcıdır ve içinde işleyici yoktur.
`internal/panel` çok parçalı bir yüklemeyi okur. Panelin ürettiği QR kodu:
bir telefonun erişim noktasına katılmak için taradığı. [`internal/panel/view.go`](https://github.com/Iman/caspian/blob/main/internal/panel/view.go) onu inşa ediyor
`qr.Encode` ve `qr.WiFiJoin` ile görüntü kitaplığı ve uzaktan hizmet sağlanmaz
dahil.



[İngilizce: HTTP/2, HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.tr#http2-and-http3-are-carried-under-a-different-name) | [English](https://github.com/Iman/caspian/wiki/Protocols-and-Transports#protocols-and-transports) | [Kodlar: HTTP/2, HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa#http2-and-http3-are-carried-under-a-different-name) | [فارسی](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa#protocols-and-transports) | [Rusça: HTTP/2, HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru#http2-and-http3-are-carried-under-a-different-name) | [Русский](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru#protocols-and-transports) | [Kaynak: HTTP/2, HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh#http2-and-http3-are-carried-under-a-different-name) | [中文](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh#protocols-and-transports)

<!-- Caspian guide navigation -->

Caspian kılavuzları: [kurulum ve desteklenen protokoller](https://github.com/Iman/caspian/wiki/Home.tr) · [DPI'yı aşmak için SNI sahtekarlığı: kurulum ve sınırlar](https://github.com/Iman/caspian/wiki/SNI-Spoofing.tr).


<!-- English-source-sha256: caae2c1c2ed8f7b292b28b1371b95851d6f133b60ac1e202d1b6a74a314aeadc -->
