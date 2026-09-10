<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Security-and-Privacy) | [فارسی](https://github.com/Iman/caspian/wiki/Security-and-Privacy.fa) | [Русский](https://github.com/Iman/caspian/wiki/Security-and-Privacy.ru) | [中文](https://github.com/Iman/caspian/wiki/Security-and-Privacy.zh) | [العربية](https://github.com/Iman/caspian/wiki/Security-and-Privacy.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Security-and-Privacy.tr) | [اردو](https://github.com/Iman/caspian/wiki/Security-and-Privacy.ur)

</div>

<a id="security-and-privacy"></a>
# Güvenlik ve gizlilik



[Caspian wiki'si](https://github.com/Iman/caspian/wiki/Home.tr)

> Bu kılavuz mevcut README'den alınmıştır. Ölçümleri orijinal tarihlerini koruyor; bu belgeleme hamlesi yeni bir test çalıştırmasını bildirmez.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="what-it-guarantees"></a>
## Neyi garanti eder

Buradaki her başlık, içinde oluşturulan güvenlik duvarı çıktısıyla desteklenir.
`internal/netcfg/testdata/`, adlandırılmış bir testle veya
depo. [`docs/BEHAVIOUR.md`](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md) vaatlerin okunabilir listesidir. Her
başlığı `test/bdd/`'deki bir senaryonun adıdır ve her senaryonun bir adı vardır.
eşleşen enjekte edilen kusur. Yani "bu test iddia ettiği şeyi tespit edebilir
tespit"in kendisi bir test sonucudur.

<a id="forwarded-client-traffic-fails-closed-and-the-block-does-not-need-the-tunnel"></a>
### İletilen istemci trafiği kapatılamaz ve bloğun tünele ihtiyacı yoktur

İleri zincirin politikası `drop`'dir. Buradaki ilk kural sızıntının engellenmesidir,
ve yalnızca etkin noktayı ve yukarı bağlantıyı adlandırır:

iifname "wlan0" oifname "eth0" yorumu bırak "başarısızlıkla kapatıldı: istemci trafiği hiçbir zaman yukarı bağlantıdan ayrılmaz"

İstemci trafiğine izin veren her kural, tünel cihazını adlandırır;
tünel kaybolur, bu kurallar eşleşmeyi bırakır ve politika her şeyi bırakır.
Tünel gittiğinde bloğun kendisi çalışmayı durduramaz çünkü
Bahset. Her arayüz ada göre eşleştirilir, asla dizine göre eşleşmez, bu nedenle
Kural seti, tünel mevcut olmadan yüklenir; bu da tam olarak ihtiyaç duyulduğu zamandır.
yönlendirme sonrası zinciri bilerek boş bırakılmıştır.

Senaryo: "Tünel gittiğinde hiçbir şey istemci trafiğinin yukarı bağlantıdan çıkmasına izin vermez".
Destekleyici analizör testi: `TestWithoutInterfaceRemovesOnlyTheRulesNamingIt`.

<a id="the-kill-switch-covers-the-boxs-own-traffic-too"></a>
### Kill switch kutunun kendi trafiğini de kapsıyor

Çıkış zinciri, adlandırılmış bir izin listesine sahip `policy drop`'dir. İzinler şunlardı:
örnekleme yerine hedefte gerçekte neyin çalıştığını numaralandırarak türetilir
trafik ve oluşturulan kural kümesindeki her izin, şu okumayı taşır:
bunu haklı çıkarıyor: NetworkManager'ın DHCP istemci yuvaları, systemd-timesyncd, DNS,
geridöngü, tünel cihazı, IPv6 komşu keşfi ve proxy sunucusu
bağlantı noktası yerine **adrese** göre izin verilir, bu nedenle 443 üzerinde UDP aktarımına izin verilmez
sessizce kırıldı. Ölçme yerine akıl yürütmeden bir izin eklendi ve
öyle diyor: DHCP'yi bağlantı noktasını izleyen, etkin noktadaki bir sunucu olarak yanıtlayan kutu
DHCP yanıtı ve isteği hiçbir tuple paylaşmadığı için kapsanamaz.

Provokasyonlar ve daha da önemlisi olumsuz kontroller kayıt altına alınıyor
`PROVENANCE.md`, "Üç provokasyon, politika yüklü olarak çalıştırılıyor". Testler:
`TestRestrictedEgress_PermitList`,
`TestRestrictedEgress_AcceptsEstablishedBeforeItDropsAnything`,
`TestRestrictedEgress_ServerIsPermittedByAddressNotPort`.

Maliyet, keşfedilmek yerine kural kümesinin kendi başlığında belirtilir: `apt
Cihaz açıkken kutudaki bir kabuktan güncelleme başarısız oluyor.

<a id="there-is-an-emergency-cut-that-does-not-take-the-hotspot-with-it"></a>
### Sıcak noktayı yanına almayan bir acil durum kesintisi var

Cihazı kapatmak, telefonun bağlantısını kesen erişim noktasını kapatır
düğmeyi basılı tutuyoruz. Yani iletilen istemciyi bırakan ayrı bir kontrol var
sıcak noktayı, DHCP'yi, DNS'yi ve paneli açık bırakırken trafik. Bkz.
[`internal/privsvc/cut.go`](https://github.com/Iman/caspian/blob/main/internal/privsvc/cut.go) ve [`internal/panel/priv.go`](https://github.com/Iman/caspian/blob/main/internal/panel/priv.go)'deki panel eylemi `cut`.

Kesim, çalışma zamanı durumudur ve hiçbir zaman diske yazılmaz, bu nedenle fişi çekin
onu geri alır. Testler: `TestCuttingClientTrafficLeavesTheWayBack`,
`TestACutIsNeverWrittenDown`, `TestACutDoesNotSurviveARestart`,
`TestForwardCut_StopsClientsAndKeepsThePanelReachable`.

Kontroller ve her birinin neyi durdurduğu için [Panel ve konfigürasyon](https://github.com/Iman/caspian/wiki/Panel-and-Configuration.tr)'ye bakın.

<a id="it-reads-the-interface-back-from-the-kernel-instead-of-trusting-a-process"></a>
### Bir sürece güvenmek yerine arayüzü çekirdekten okur

Başlatılmış bir süreç, işe yaradığının kanıtı değildir. Bu gerçek bir başarısızlıktı.
[`internal/privsvc/readback.go`](https://github.com/Iman/caspian/blob/main/internal/privsvc/readback.go) başlığı, 2026-08-30 tarihinde hizmetin kaydedildiğini kaydediyor
`wlan0` üzerinde bir erişim noktasıyla çalıştığını kaydederken, `wlan0` hala bir
ev ağındaki istasyon. hostapd, kontrol soketi olan canlı bir süreçti.
cevap vermedi. Odadaki bir telefon bizimkinin dışında on bir şebekeyi listeliyordu.
onlar. Ve dnsmasq başka birinin LAN'ındaki bir yabancının cihazına yanıt veriyordu
bir DHCPNAK.

kadar hiçbir şeyin sıcak nokta arayüzüne bağlanmasına izin verilmez.
`netcfg.AssertHotspotInterfaceReleased` ücretsiz olduğunu kanıtlıyor ve hiçbir şey rapor edilmiyor
`AssertHotspotIsAccessPoint` bir erişim noktasını okuyana kadar kendisi çalışıyor
Beklenen isim yayınlanıyor. Testler:
`TestNothingBindsToTheHotspotInterfaceUntilItIsProvedFree`,
`TestTheServiceDoesNotReportRunningUntilTheAccessPointReadsBackAsOne`,
`TestAnAccessPointBroadcastingAnotherNameIsNotOurs`,
`TestTheReleaseIsReadBackBeforeAnythingBindsAndTheAccessPointAfter`.

<a id="you-get-your-wifi-back"></a>
### WiFi'nizi geri alırsınız

Her ağ değişikliği, `/var/lib/caspian/netcfg.journal`'ye kendi koduyla birlikte yazılır.
ters **yapılmadan önce** ve kapatıldığında bunlar tersten tekrar oynatılır.
kayıt bellekte değil diskte olduğundan, sonlandırılan bir işlem veya elektrik kesintisi
onu kaybetme. Değişimin ortasında ölen bir kutu, plağa bakmadan önce kaydı tekrar oynatıyor
makine veya yeni bir şey uygular.

Bir WiFi arayüzünün devralınması adım adım günlüğe kaydedilir. İleri
dizisi ve bunun tersi hedef üzerinde çalıştırılmış ve kaydedilmiştir.
`PROVENANCE.md`, "Serbest bırakma dizisi hedefte çalıştırıldı". Dört
komutlar gitti ve tersler kutuyu kendi ağına geri koydu
sekiz saniye sonra kendi adresi.

Bir değişikliğin kasıtlı olarak tersi yoktur ve
`TestPlan_InvariantsHoldOnEveryModelledMachine` bunun tek olduğunu iddia ediyor:
sıcak nokta arayüzünü getiriyoruz. Çıkarken radyoyu kapatmak daha kötü
onu bırakmaktansa, çünkü makinenin kendi WiFi'si ve kullanıcının bulunduğu panel
okuyor, üzerinde olabilir.

Senaryolar: "anahtarı kapatmak kutuda yapılan her değişikliği geri getirir", "a
Kapatılan bir sürecin günlüğünden tekrar oynatılan parçalama aynı değişiklikleri geri alır",
"Yarı yolda öldürülen bir kutu, başka bir şey yapmadan önce temizlenir". Testler:
`TestJournal_RecordsInverseBeforeTheChange`,
`TestTeardown_ReplaysInExactReverseOrder`,
`TestRecover_UndoesAJournalLeftByAKilledProcess`,
`TestTheTakeoverReleasesTheInterfaceItSaysItWillRelease`.

Tersi başarısız olursa, güvenlik duvarının kendi tersi çalıştırılmak yerine **tutulur**, dolayısıyla
rotasını geri alamayan bir kutu bloğunu korur. Test:
`TestTheFirewallIsNotRemovedWhenAnEarlierInverseFailed`.

<a id="the-pasted-config-never-reaches-a-screen-a-log-or-a-readable-file"></a>
### Yapıştırılan yapılandırma hiçbir zaman bir ekrana, bir günlüğe veya okunabilir bir dosyaya ulaşmaz

Akışın ürettiği her şey yapıştırılan kimlik bilgisi için aranır:

- kullanıcıya gösterilen her hata ve her mesaj
- cihazın günlük satırları
- panelin yapılandırma açıklaması
- teşhis için oluşturulurken kaydedilen ayarlar
- oluşturulan güvenlik duvarı
- oluşturulan DHCP ve DNS yapılandırması
- motorun kendi günlüğü
- diskteki günlük
- Ayrıcalıksız panelden ayrıcalıklı hizmete geçen istek

Hiçbirinde yok. Olması gereken iki yerde olup olmadığı kontrol edilir, yani
test, yapılandırmanın kaybolmasından geçemez.

Senaryolar: "yapıştırılan kimlik bilgileri hiçbir zaman bir ekrana, bir günlüğe veya okunabilir bir dosyaya ulaşmaz
file", "hotspot şifresi erişim noktasına ulaşıyor, başka bir şey yok". Testler:
`TestPastedConfigNeverAppearsInAResponseOrALog`,
`TestFailedConfigPathsDoNotEchoTheInput`, `TestStartRequestRedactsItself`,
`TestNoCredentialReachesTheAdvancedView`,
`TestTheServerAddressNeverAppearsInADiagnosticLine`.

<a id="the-panel-asks-the-internet-for-nothing-on-its-own"></a>
### Panel internetten tek başına hiçbir şey istemiyor

Tarayıcının yüklediği her stil sayfası, komut dosyası ve simge ikili dosyada derlenir
`go:embed` ile. Bkz. [`internal/panel/assets.go`](https://github.com/Iman/caspian/blob/main/internal/panel/assets.go). Hiçbir web yazı tipi yok:
stil sayfasının yazı tipi yığını tamamen sistem yüzlerinden oluşur, Farsça özellikli olanlar
ilk.

Gizlilik nedeni, uzaktaki bir varlığın üçüncü bir tarafa adresini söylemesidir.
paneli açan herkes. Daha güçlü neden ise kullanılabilirliktir. Panel var
Tünel çöktüğünde yükleme yapmak için, yani tam da birisinin buna ihtiyacı olduğunda.

Bir değil iki mekanizma. `TestNoAssetReferencesAnExternalURL` ve
`TestNoRenderedPageReferencesAnExternalURL` varlıkları ve oluşturulan her şeyi tarar
mutlak bir URL için sayfa. `setSecurityHeaders`, `default-src 'none'`'yi şununla gönderir:
Listelenen her kaynak `'self'` olarak ayarlanmıştır, dolayısıyla tarayıcı bu sınırı geçen kaynağı reddeder.
testler. `internal/panel`'nin herhangi bir yerinde kendi dışında hiçbir giden HTTP istemcisi mevcut değil.
kendi testleri.

Caspian'ın içerik için yaptığı tek istek aboneliğin yenilenmesidir ve
kişi ona basar. Panel yerine ayrıcalıklı yarıda çalışır.
Motor çalışmadığı ve aranmadığı sürece herhangi bir priz açılmadan reddedilir
motorun kendi geri döngü SOCKS'u aracılığıyla, verilen ana bilgisayar adı ile birlikte gelir
burada çözümlenmek yerine proxy, dolayısıyla ne bayt ne de ad araması
kişinin ISP'sine ulaşın. Zamanlayıcı yok, önyükleme sırasında yenileme yok ve yenileme yok
tünel geldiğinde.

Oluşturulan yapılandırma ayrıca hiçbir yerde Google çözümleyicisinin adını vermez ve hiçbir Google çözümleyicisini kullanmaz.
`geoip:` veya `geosite:` kuralı, çünkü her ikisi de indirme işlemini bir bilgisayara yeniden başlatacaktır.
tüm yükleme öyküsü doğrulanmış bir ikili program olan ürün. Testler:
`TestNoGoogleAnywhereInGeneratedConfigs`,
`TestGoogleResolverIsRejectedAtTheSource`. Senaryo: "kutunun indirilmesine gerek yok
ve hiçbir Google sunucusuna hiçbir şey sormaz".

<a id="privilege-is-split"></a>
### Ayrıcalık bölünmüş

`caspian serve --privileged` kök olarak çalışır ve rotaların, güvenlik duvarının,
erişim noktası ve motor. Belirli bir süre boyunca adlandırılmış eylemlerin kısa bir listesini kabul eder.
unix soketi ve asla kullanıcı girdisinden oluşturulan bir komut değil. `caspian serve --panel`
Ayrıcalıksız `caspian` hesabı olarak çalışır ve web arayüzünün sahibidir.
başka bir şey değil. Kelime dağarcığı ve çerçeve formatı için [Architecture](https://github.com/Iman/caspian/wiki/Architecture.tr)'ye bakın.

Panel şifresi argon2id ile hash edilmiştir. Bkz. [`internal/state/password.go`](https://github.com/Iman/caspian/blob/main/internal/state/password.go). o
kutunun üzerindeki yerel bir şifredir. Başka hiçbir yerde hesap yoktur.

<a id="the-clock-is-checked-before-anything-handshakes"></a>
### Herhangi bir el sıkışmadan önce saat kontrol edilir

Pi'nin pil saati yoktur ve iki ayrı mekanizma duvar saatine bağlıdır.
REALITY bunu el sıkışmaya yazar ve xray-core'u yapılandıran **kabul eder**
tarihe bağlıdır. Yani saati yanlış gelen bir kutu sadece başarısız olmakla kalmaz
bağlanın. Saat ayarlandığında aynı ikilinin reddettiği bir yapılandırmayı kabul eder.
düzeltildi.

Kontrol, doğrulamadan önce ve herhangi bir şey yapılmaya çalışılmadan önce gerçekleştirilir. Bkz.
[`internal/privsvc/clock.go`](https://github.com/Iman/caspian/blob/main/internal/privsvc/clock.go), `Service.Start`'den 1. adım olarak çağrıldı.
`applyLocked`. Panelin kullanıcıyı suçlamaması için belirgin bir hata ortaya çıkarır
yapılandırma Test: `TestClockFailureIsNotBlamedOnTheConfig`.

<a id="three-config-failures-are-told-apart"></a>
### Üç yapılandırma hatası birbirinden ayrı anlatılıyor

"Bağlantı okunamadı", "okudum, yazıldığı gibi kullanılamaz" ve
"bağlantı iyiydi ve sunucu yanıt vermedi" ifadesinin üç farklı eyleme ihtiyacı var
kullanıcıdan gelir ve üçüncüsü en yaygın olanıdır. İlk önce yapılandırmayı suçlamak
birinin asla bozulmamış bir yapılandırmayı atmasına neden olur. Makinede hiçbir şey yok
yapıştırılan metin okunmadan önce dokunulur. Senaryolar: "bağlantı olmayan metin
herhangi bir şeye dokunulmadan önce kesinlikle reddedilir", "motorun yapmayacağı bir bağlantı
kabul et, ayrıştırılmayacak olanın dışında bir şey söylenir", "sunucusu asla
cevaplar bağlantıya yüklenmiyor".

<a id="what-it-does-not-guarantee"></a>
## Neyi garanti etmez

Bu liste yakından okunması gereken listedir.

<a id="dns-over-https-on-port-443-is-carried-not-blocked-and-nothing-here-can-see-it"></a>
### 443 numaralı bağlantı noktasında HTTPS üzerinden DNS taşınır, engellenmez ve burada hiçbir şey göremez

53 numaralı bağlantı noktasındaki istemci DNS'si bu kutuya her iki protokol üzerinden yönlendirilir.
sadece izin veriliyor. Dolayısıyla, sabit kodlanmış çözümleyiciye sahip bir cihaz burada yanıtlanır.
kullanması söylenen kişiye ulaşmasına izin verildi. 853'te TLS üzerinden DNS
TCP sıfırlamasıyla reddedilir, böylece cihaz yeniden yönlendirilen bağlantı noktasına geri döner. DNS
853'te QUIC üzerinden bırakıldı.

443 numaralı bağlantı noktasındaki HTTPS üzerinden DNS, diğer HTTPS'lerden ayırt edilemez ve
diğer herhangi bir şey gibi tünelden geçirildi. Bunu kullanan bir müşteri içeride
tünel ve sızıntı yapmıyor. Aynı zamanda görünmezdir. Bu projede hiçbir şey yok ve
donanım donanımındaki hiçbir şey bunu gözlemleyemez. Bu tasarımın bir sınırıdır.
Oluşturulan kural kümesinin kendisinde, [`docs/BEHAVIOUR.md`](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md)'de ve
DNS sızıntı kontrolünün çıktısını yalnızca burada değil.

<a id="ipv6-is-blocked-and-the-ipv6-path-is-not-finished"></a>
### IPv6 engellendi ve IPv6 yolu tamamlanmadı

IPv6 tüneli yoktur. Çalışan bir IPv6 yoluna sahip bir cihaz, bunu IPv4'e tercih eder
ve tüneli tamamen atlayacaktır, dolayısıyla varsayılan politika engellemektir. Dört
işler bunu tutuyor. `IPv6Block`, `netcfg.DefaultOptions`'de varsayılandır. Kutu
IPv6'yı iletmez. Güvenlik duvarı, her ikisinde de etkin noktaya iletilen IPv6'yı bırakır
yönler. Ve sıcak noktaya yönelik yönlendirici reklamları bırakılır, böylece
cihaz kendisine bir adres veremez. Senaryo: "müşterilere hiçbir zaman teklif edilmiyor
IPv6 tünel taşıyamaz".

`IPv6Forward` bir seçenek olarak mevcut ve kendi yorumu bunun ayarlanmaması gerektiğini söylüyor.
motorun TUN girişinin hedefe IPv6 taşıdığı gösterilmemiştir. Aynı zamanda
ileri zincire kasıtlı olarak hiçbir izin kuralı eklemez: IPv4 izinlerin ismine izin verir
her iki yönde de etkin nokta alt ağı olduğundan, herhangi bir yerde v6 öneki yoktur.
adlandırmayı planlıyoruz ve yalnızca iki arayüz adıyla eşleşen bir kural, herhangi bir adı kabul edecektir.
Bir müşterinin yazdığı kaynak adresi. `TestRuleset_NoUnconstrainedIPv6AcceptInForward`
bu çizgiyi koruyor.

**"Engellendi" DNS ile değil, yönlendirmeyle ilgilidir ve aradaki fark önemlidir.**
Katılan bir cihazdan gelen AAAA sorgusu bastırılmaz ve boş yanıtlanmaz. o
tünelden geçerek motora gidiyor ve gerçek AAAA kayıtlarıyla geri geliyor,
çünkü motor belgesi `UseIP`'yi ister ve dnsmasq, `filter-AAAA`'yi ayarlamaz.
Bu nedenle bir cihaz, hiçbir şekilde erişmesinin mümkün olmadığı IPv6 adreslerini öğrenir ve geri çekilir.
IPv4'e.

Bu zararsızdır, ancak hiçbir şey müşteriye v6 adresi veremez ve
Herhangi bir şey olursa zararsız olmayı bırakan ilk şey, çünkü bir müşteri
çalışan bir v6 yolu ile AAAA cevabını tercih ediyor ve bu yoldan ayrılacak
kutu taşımamaktadır. Sürpriz olarak bırakılmak yerine buraya yazıldı ve
`TestAAAAQueriesAreAnsweredAndNotSuppressed` her iki yarıyı da sabitler, böylece
bir karar olması gerekiyor.

**Donanım donanımı IPv6'yı hiçbir şekilde derecelendiremez, dolayısıyla bundan IPv6 sonucu alınamayacağı anlamına gelir
her şey.** [`test/hardware/README.md`](https://github.com/Iman/caspian/blob/main/test/hardware/README.md), "Bu bakış açısının yapamayacağı şeyler" başlığı altında kaydeder
not: IPv6", telefonun yalnızca yerel bağlantı adresi taşıdığını, yani
`ip -6 route show default` telefonda ve Pi'de boştur ve bu
IPv6'ya bağlantı değişmez yanıtı "Ağa ulaşılamıyor". IPv6 yok
o LAN'da hiç değilse, oradaki bir IPv6 sızıntı kontrolü çalışması,
cihaz herhangi bir şey yapıyor. Bu projenin sahip olduğu her donanım sonucu bir IPv4'tür
sonuç. Bunu IPv6 çalışan bir ağda çalıştıran herkes bunu bir sorun olarak ele almalıdır.
yeni bir soru, kapalı bir soru değil ve testi yazmak yerine testi yazmayı beklemelisiniz
birini etkinleştirin.

<a id="the-boxs-own-traffic-is-outside-the-fail-closed-promise-by-design"></a>
### Kutunun kendi trafiği, tasarım gereği arızalı kapatma vaadinin dışındadır

Söz **yönlendirilen müşteri trafiği** ile ilgilidir. Kutunun kendi bağlantısı
sunucunuzun yukarı bağlantıya doğrudan ulaşması gerekiyor veya hiç tünel yok ve
[`docs/2026-08-29-design.md`](https://github.com/Iman/caspian/blob/main/docs/2026-08-29-design.md) bölüm 7, kutunun kendi trafiğini kutunun dışına koyuyor
bu nedenle garanti.

Çıkış zinciri öldürme anahtarı bunu daraltır ve kapatmaz. Oluşturulan
kural kümesi, artığı kendi başlığında belirtir. DNS bir deliktir: üzerinde herhangi bir şey
kutusu hala 53 numaralı bağlantı noktasından ağa ulaşabiliyor ve sunucunun ana bilgisayar adı
herhangi bir tünel oluşmadan önce yerel ağda net bir şekilde çözüldü. İkisi de değil
istemci trafiğinin sızması ve kill switch'in bu durumu daha da kötüleştirmesi.

Giriş zincirinin politikası yine tasarım gereği `accept`'dir ve ayrıca belgede de belirtilmiştir.
kural seti. Daha önceki bir versiyonda `drop` olarak vardı ve `PROVENANCE.md` neyi kaydeder
hedefte ölçüldüğünde meydana geldi. Her yeni gelen bağlantı
reddetti ve SSH yanıt vermeyi durdurdu, bu arada zaten açık olan oturum çalışmaya devam etti,
başsız bir makinede bir kazadan ayırt edilemez. Tek yer
giriş zinciri, birleştirilmiş bir cihazın bulunduğu sıcak nokta tarafı olan her şeyi kısıtlar
DHCP'ye, DNS'ye, panele ve ICMP yankısına ulaşır ve kutuda başka hiçbir şeye ulaşmaz.

<a id="client-isolation-is-a-rule-not-a-measurement"></a>
### Müşteri izolasyonu bir ölçüm değil kuraldır

Kural kümesi `iifname "wlan0" oifname "wlan0" drop`'yi içerir. Kural şudur
mevcut olup olmadığı kontrol edilir. İşe yaraması değil.

<a id="nothing-in-this-repository-captures-an-exit-ip"></a>
### Bu depodaki hiçbir şey çıkış IP'sini yakalayamıyor

`test/tunnel`, gerçek baytları gerçek bir xray-core sunucusu aracılığıyla taşır ve her şey
içinde geri döngüde olduğundan çıkış IP'si yakalayamaz ve yakalayamaz. `test/bdd`'de hayır
ağ yok, radyo yok, kök yok ve tünel cihazı yok. Gerçek motoru çalıştırıyor
gerçek yapılandırma yükleyicisi aracılığıyla işlem yapar, dolayısıyla "motor bu yapılandırmayı kabul etti ve
"Başladı" ne söylediği anlamına gelir, ancak gelen tünel kapalıdır.

Dolayısıyla bu depodaki hiçbir şey projenin kendi çağrı standardını karşılamıyor
çalışan bir şey. [`docs/BEHAVIOUR.md`](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md) şu bölümle bitiyor: "Bu paketin özellikleri nelerdir?
kanıtlamıyor", hala borçlu olunanları listeliyor. Paketin bir parçası olarak okuyun.

<a id="nothing-re-checks-the-firewall-once-it-is-loaded"></a>
### Güvenlik duvarını yüklendikten sonra hiçbir şey yeniden kontrol etmez

Bkz. [kusur D1](https://github.com/Iman/caspian/wiki/Troubleshooting.tr). Cihaz çalışırken masanın üzerine bir şey sıçrarsa
Çalışıyor, kutu iletilmeye devam ediyor, panel raporlamayı bağlı tutuyor ve
hiçbir şey fark edilmiyor.

<a id="nothing-watches-the-uplink"></a>
### Yukarı bağlantıyı hiçbir şey izlemiyor

Kablonun çıkarılması veya kira sözleşmesinin yenilenmesi nedeniyle internet hareketleniyor
farklı olarak, hiçbir şeyin yüksek sesle başarısızlıkla sonuçlanmadığı bir değişikliktir. Sabitlenmiş rota
sunucu hâlâ mevcut ve hâlâ bir çıkış yolu olmayan bir adresi işaret ediyor.
Kutu bunu fark etmiyor ve birisi düğmeye basana kadar tünel kapalı kalıyor.
tekrar geçiş yapın.

Bunun maliyeti mahremiyet değil, kullanılabilirliktir. İstemci trafiği engellenmiş durumda kalıyor
baştan sona, çünkü ileri politika düşmedir ve içindeki her kabul,
tünel. `netcfg.WatchUplink` ve `Plan.RederiveForUplink` mevcut ve çalışıyor;
gönderilen kod çağrıları da. `TestNothingInTheApplianceWatchesTheUplink` nedir
karşıt cümlenin olduğu yere, belgelere sürüklenmesini durdurur
2026-08-30'a kadar.

<a id="mode-b-has-never-been-run-on-real-hardware"></a>
### B Modu hiçbir zaman gerçek donanımda çalıştırılmadı

Her mod B fikstürü yazılmıştır. `PROVENANCE.md`, hedefin sahip olduğunu kaydediyor
tek bir radyo var ve USB adaptörü yok, bu nedenle bu ürünün insanlara söylediği düzenleme
için bir adaptör satın alın, kimsenin ölçmediği baytlara karşı kanıtlanmıştır.



[Architecture](https://github.com/Iman/caspian/wiki/Architecture.tr) | [Panel-and-Configuration](https://github.com/Iman/caspian/wiki/Panel-and-Configuration.tr) | [Troubleshooting](https://github.com/Iman/caspian/wiki/Troubleshooting.tr)

<!-- SNI upstream credits -->

SNI kimlik sahtekarlığı kredileri: Windows x64'te WinDivert (LGPL-3.0) ile [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing) (GPL-3.0).
[Üçüncü taraf lisanslar, kaynak sürümleri ve krediler](https://github.com/Iman/caspian/blob/feature/sni/docs/THIRD-PARTY.md).

<!-- Caspian guide navigation -->

Caspian kılavuzları: [kurulum ve desteklenen protokoller](https://github.com/Iman/caspian/wiki/Home.tr) · [DPI'yı aşmak için SNI sahtekarlığı: kurulum ve sınırlar](https://github.com/Iman/caspian/wiki/SNI-Spoofing.tr).


<!-- English-source-sha256: 535cf4665f69f332fe7b3455f5d65126b1a3ebbe759ae17c46983ea6ab766450 -->
