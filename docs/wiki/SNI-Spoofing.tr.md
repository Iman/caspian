<!-- wiki-navigation:start -->
<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/SNI-Spoofing) · [فارسی](https://github.com/Iman/caspian/wiki/SNI-Spoofing.fa) · [Русский](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ru) · [简体中文](https://github.com/Iman/caspian/wiki/SNI-Spoofing.zh) · [العربية](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ar) · [**Türkçe**](https://github.com/Iman/caspian/wiki/SNI-Spoofing.tr) · [اردو](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ur)

</div>

<div dir="ltr" lang="tr">

[Caspian vikisi](https://github.com/Iman/caspian/wiki/Home.tr) · [Sorun giderme](https://github.com/Iman/caspian/wiki/Troubleshooting.tr)

</div>
<!-- wiki-navigation:end -->

<a id="caspian-sni-spoofing-and-tls-splitting-for-dpi-circumvention"></a>
# Caspian SNI sahtekarlığı ve DPI'yı atlatmak için TLS bölme

Bu kılavuz, mevcut olarak yayımlanan yükleyiciyi değil, `feature/sni`'deki isteğe bağlı derin paket incelemesini (DPI) atlatmayı kapsar.

SNI, TLS karşılamasındaki sunucu adıdır.
Bu özellik, gerçek proxy akışından önce sahte bir adla ekstra bir karşılama mesajı gönderir.
İçe aktarılan yapılandırmanızdaki gerçek TLS veya REALITY adı değişmeden kalır.
İçe aktarılan bir `sni` değeri, sahteciliği otomatik olarak etkinleştirmez.

<a id="set-a-spoof-name"></a>
## Sahte bir ad belirleyin

1. Proxy yapılandırmanızı panele kaydedin.
2. Kontrol panelinde **DPI atlatmayı (isteğe bağlı)** açın.
3. Yerel test için `cover.example.invalid` gibi bir alan adı girin.
4. Ayarı kaydedin.

Gerçek ağınız için uygun bir alan adı kullanın; örnek etki alanı çözümlenemiyor.
Caspian açıksa, kaydetme onu yeni ayara yeniden bağlar.
Sahteciliği devre dışı bırakmak için alanı temizleyin ve kaydedin.
Yapılandırmanın değiştirilmesi sahte adı ve her iki bölünmüş ayarı temizler.
Başka bir giriş seçmek veya bir aboneliği yenilemek onu korur.

<a id="independent-tcp-split-and-tls-record-split"></a>
## Bağımsız TCP bölünmesi ve TLS kaydı bölünmesi

Kayıtlı yapılandırmanızın yanında **DPI atlatmayı (isteğe bağlı)** açın:

- **Sahte sunucu adı**: Sahte SNI'yi kapatmak için boş bırakın.
- **TCP bölünmesi**: İlk TLS karşılamasını SNI ana bilgisayar adının ortasına yakın bir yerde iki yazma halinde gönderin.
- **TLS kaydı bölme**: el sıkışma içeriğini değiştirmeden TLS karşılama kaydını bu konumda bölün.

Her seçenek bağımsızdır; bunları birleştirebilir veya üçünü birden bırakabilirsiniz.
SNI uzantısı olmadan bölme işlemi, el sıkışma türünden hemen sonraki konumu kullanır.
TCP bölmek en iyi çabadır: ayrı yazmalar her işletim sistemi ve ağda ayrı paketleri garanti etmez.
TLS kaydını bölme, kayıt sınırlarını değiştirir ve bazı sunucularda veya CDN'lerde başarısız olabilir.
Her iki bölünmüş seçenek de şu anda desteklenen IPv4 TCP aktarımları üzerinden VLESS, VMess veya Trojan ile sıradan TLS gerektirir.
REALITY bölme etkin değil; REALITY yine de sahte SNI'yı tek başına kullanabilir.
Yalnızca ilk ClientHello bölünür; daha sonra uygulama trafiği normal şekilde iletilir.
Yalnızca bölünmüş modun WinDivert, paket soketi veya BPF erişimine ihtiyacı yoktur.
Sahte SNI da seçildiğinde, herhangi bir gerçek selamlama gönderilmeden önce paket onayının tamamlanması gerekir.
Yanlış biçimlendirilmiş, eksik, büyük boyutlu veya durmuş karşılama mesajları bağlantıyı kapatır; yeniden deneme yok etkinleştirilmiş bir seçeneği sessizce kaldırır.
Kaydetme etkin bir tüneli yeniden bağlar; yapılandırma değişikliği üç seçeneği de siler.

Durum alanları `spoof_sni`, `tcp_split` ve `tls_record_split`'dir.
Sürüm 4 dosyasını yükseltmek sahte adını korur ve her iki bölme ayarını da kapalı bırakır.

<a id="state-compatibility"></a>
## Durum uyumluluğu

Bu dal durum şeması sürüm 5'i yazar.
Daha eski yapılar, veri kaybını önlemek için bu durum dosyasını reddeder.
Daha eski bir yapıya geri dönmeniz gerekiyorsa, yükseltme öncesi durum yedeğini tutun.

<a id="limits"></a>
## Sınırlar

Bu sürüm, WebSocket, HTTPUpgrade ve gRPC dahil olmak üzere IPv4 TCP üzerinden VLESS, VMess ve Trojan'yi destekler.
Hysteria2, QUIC, SOCKS, Shadowsocks, XHTTP veya yalnızca IPv6 sunucularını desteklemez.
SOCKS, bu TCP ileticisinin kapsayamayacağı ayrı bir UDP uç noktası üzerinde anlaşabilir.
Tünel DNS trafiği de dahil olmak üzere yerel Shadowsocks UDP, bu yalnızca TCP ileticisini kullanamaz.
İletici, tespit edilen sunucu adresleri arasından kullanılabilir ilk IPv4 adresini seçer.
Onay başarısız olursa gerçek akışı iletmeden bağlantıyı kapatır.
Sıradan bir bağlantıya dönüşmez.

Windows x64'teki Sahte SNI, yerel yükleyici yapısında bulunan WinDivert dosyalarına ihtiyaç duyar.
Windows ARM64 sahte SNI kullanamaz; salt bölme modu paket arka ucunu kullanmaz.
Linux'ta sahte SNI'nın paket soket ayrıcalıklarına ihtiyacı vardır; macOS'un Ethernet tarzı bir arayüz üzerinden bir BPF cihazına erişmesi gerekiyor.
Ayrıcalıklı Caspian servisi bu kaynakların sahibidir ve durduğunda bunları kapatır.

Windows geri döngü testi, sunucunun gerçek akışı değişmeden aldığını kanıtlar.
Kimlik sahtekarlığının sağlayıcınızın filtrelemesine aykırı olduğunu kanıtlamaz.
Linux ve macOS sürümlerinin de hedef sistemlerinde canlı paket testlerine ihtiyacı vardır.

<a id="credits"></a>
## Kredi

Birincil kaynak ve fikir, GPL-3.0 kapsamında lisanslanan [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing)'dir.
Bkz. [üçüncü taraf kredileri](https://github.com/Iman/caspian/blob/feature/sni/docs/THIRD-PARTY.md).

<a id="dpi-bypass-sni-spoofing-and-security"></a>
## DPI bypass, SNI sahtekarlığı ve güvenlik

<a id="is-caspian-dpi-safe"></a>
### Caspian DPI güvenli mi?

Evrensel bir DPI güvenliği garantisi yoktur. İsteğe bağlı SNI sahtekarlığı, bir filtreleme sisteminin ilk TCP trafiğini nasıl okuyacağını etkilemeye çalışır.
Sunucu IP'sini, trafik hacmini veya zamanlamasını gizlemez ve sağlayıcı bağlantıyı yine de engelleyebilir.
`feature/sni` uygulaması gerçek TLS kimliğini korur ve gerçek akışı doğrudan göndermek yerine başarısız olan sahtekarlık onayını reddeder.

<a id="does-caspian-include-goodbyedpi-or-zapret"></a>
### Caspian, GoodbyeDPI veya zapret'i içeriyor mu?

Hayır. [GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI) ve [zapret](https://github.com/bol-van/zapret), gelecekteki olası stratejiler için araştırma referanslarıdır.
Birincil SNI kodu ve fikri, GPL özelliği korunarak [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing)'den gelmektedir.
Caspian bu diğer projeleri bir araya getirmez veya yazarlarının bunu desteklediğini iddia etmez.

<a id="does-a-config-with-sni-enable-dpi-bypass-automatically"></a>
### SNI'lı bir yapılandırma DPI bypass'ı otomatik olarak etkinleştirir mi?

Hayır. İçe aktarılan SNI gerçek sunucu kimliğidir. Bu modu etkinleştirmek için isteğe bağlı ayrı bir sahtekarlık adı ayarlayın.
Etkinleştirmeden önce [SNI kurulumu ve sınırlamaları](https://github.com/Iman/caspian/wiki/SNI-Spoofing.tr)'yi okuyun.

<a id="sni-idea-acknowledgements"></a>
## SNI fikir teşekkürleri

Caspian ayrıca SNI çalışmalarına bilgi sağlayan fikirler ve uygulama karşılaştırmaları için bu projelerin yazarlarına ve katkıda bulunanlara teşekkür ediyor.
Kodları ve yürütülebilir dosyaları paketlenmemiştir.

- [selfishblackberry177/sni-spoof](https://github.com/selfishblackberry177/sni-spoof): SNI yönlendirme karşılaştırmasına gidin.
- [therealaleph/sni-spoofing-rust](https://github.com/therealaleph/sni-spoofing-rust): Rust SNI uygulaması ve özellik karşılaştırması.
- [bol-van/zapret](https://github.com/bol-van/zapret): DPI atlatma stratejileri ve teşhisleri.
- [ValdikSS/GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI): DPI atlatma stratejileri.
- [Floxu1/UAC-SNI-Spoofer-Android](https://github.com/Floxu1/UAC-SNI-Spoofer-Android): Android SNI entegrasyon fikirleri.

Uyarlanmış kod, yukarı akış lisansını ve bildirimlerini korur.
Fikir onayları, kodun kopyalanmasına izin vermez veya onaylandığı anlamına gelmez.
İncelenen sürümler, lisanslar ve kullanım kapsamı için [üçüncü taraf kredileri](https://github.com/Iman/caspian/wiki/Third-Party-Credits.tr)'ye bakın.

[Doğrulama sonuçları ve kalan donanım testleri](https://github.com/Iman/caspian/blob/feature/sni/docs/SNI-VALIDATION.md).

<!-- English-source-sha256: 0c3a086c7127a1bec0deacf690b2aed7696a340ce94ec55379e1f233bee629f0 -->
