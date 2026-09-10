<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Licence-and-Credits) | [فارسی](https://github.com/Iman/caspian/wiki/Licence-and-Credits.fa) | [Русский](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ru) | [中文](https://github.com/Iman/caspian/wiki/Licence-and-Credits.zh) | [العربية](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Licence-and-Credits.tr) | [اردو](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ur)

</div>

<a id="licence-and-credits"></a>
# Lisans ve krediler



[Caspian wiki'si](https://github.com/Iman/caspian/wiki/Home.tr)

> Bu kılavuz mevcut README'den alınmıştır. Ölçümleri orijinal tarihlerini koruyor; bu belgeleme hamlesi yeni bir test çalıştırmasını bildirmez.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="licence"></a>
## Lisans

AGPL-3.0-or-later, bölüm 7 kapsamında üç ek terimle birlikte. Üçü de
Bölüm 7, yazılımla yapabileceklerinize izin verir ve hiçbiri bunları kısıtlamaz:
telif hakkı bildirimini, bu atıf ve görünür bir referansı koruyun
Caspian projesi herhangi bir kullanıcı arayüzünde; sürümünüzü değiştirilmiş olarak işaretleyin
onu değiştir; ve yazarların veya projenin adlarını tanıtım amacıyla kullanmayın,
bu isimler adına bağış, sponsorluk veya hibe talep etmek de buna dahildir.
tam metin [`LICENSE`](https://github.com/Iman/caspian/blob/main/LICENSE)'de, terimler ise [`NOTICE`](https://github.com/Iman/caspian/blob/main/NOTICE)'dedir.

Bu üçüncü terim, İSİMLERİN kullanımını kısıtlar, başka hiçbir şeyi kısıtlamaz. Sen özgür kal
AGPL kapsamındaki yazılımı herhangi bir amaçla çalıştırın, inceleyin, değiştirin ve yeniden dağıtın.
Ticari bir amacı da içeren bir amaç. Yapamayacağınız şey para toplamaktır
yazarların adı.

GPL yerine AGPL çünkü bu program normalde bir
diğer insanların bağlandığı hizmettir ve bölüm 13, düz GPL ile aradaki boşluğu kapatır
yapraklar. İzin verici bir lisans değildir, çünkü ikili dosya statik olarak bağlanır
GPL-3.0-or-later kodu: `github.com/sagernet/sing` ve
`github.com/sagernet/sing-shadowsocks`, her ikisine de xray çekirdeği yoluyla ulaşıldı. Yani
birleşik çalışma GPL ailesi koşullarında olmalıdır ve MIT veya Apache-2.0 değildir
bunun için mevcut.

<a id="built-on"></a>
## Üzerine inşa edildi

Caspian, diğer insanların çalışmalarını çevreleyen az miktarda koddur. Motor
xray-core ve paylaşım bağlantısı ayrıştırıcısı XTLS'dir. Hiçbir proje bunu onaylamıyor
bir; iş kendilerine ait olduğu için kredilendirilirler.

| Proje | Lisans | Burada ne işe yarıyor? |
|---|---|---|
| [xray-core](https://github.com/xtls/xray-core) | MPL-2.0 | Ayrı bir program olarak çalıştırmak yerine süreç içinde bağlantılı proxy motoru |
| [libXray](https://github.com/XTLS/libXray) | MIT | `third_party/libxray-share/` kapsamında sunulan paylaşım bağlantısı ayrıştırıcısı |
| [REALITY](https://github.com/xtls/reality) | MPL-2.0 | TLS kamuflaj taşımacılığı |
| [uTLS](https://github.com/refraction-networking/utls) | BSD-3-Clause | TLS parmak izi taklidi |
| [quic-go](https://github.com/apernet/quic-go) | MIT | QUIC yığını Hysteria2 çalışır |
| [gVisor](https://github.com/google/gvisor) | Apache-2.0 | Kullanıcı alanı ağı, TUN'un gelen kullanımlarını kullanır |
| [sing](https://github.com/sagernet/sing) ve [sing-shadowsocks](https://github.com/sagernet/sing-shadowsocks) | GPL-3.0-or-later | Shadowsocks 2022 ve bu projenin copyleft olmasının nedeni |
| [netlink](https://github.com/vishvananda/netlink) | Apache-2.0 | Arayüzler, adresler ve rotalar |
| [miekg/dns](https://github.com/miekg/dns) | BSD-3-Clause | DNS mesajı işleme |
| [gorilla/websocket](https://github.com/gorilla/websocket) | BSD-2-Clause | WebSocket aktarımı |
| [CIRCL](https://github.com/cloudflare/circl) | BSD-3-Clause | Kuantum sonrası anahtar değişimi |
| [Wintun](https://www.wintun.net/) | Wintun Önceden Oluşturulmuş İkili Lisans | Windows'ta imzalı `wintun.dll` tünel sürücüsü |
| [.NET çalışma zamanı ve Windows Forms](https://github.com/dotnet/runtime) | MIT | Bağımsız Windows yardımcısı ve tepsi uygulaması çalışma zamanı |
| `System.ServiceProcess.ServiceController` | MIT | `CaspianControl.exe`'den Windows hizmet kontrolü |

Windows kurulumu `wintun.dll`'yi içerir. SNI yapısı ayrıca Windows x64'te WinDivert'i de içerir.
Caspian, resmi imzalı Wintun 0.14.1 ikili dosyasını değişiklik yapmadan dağıtır.
Lisansı şuradadır
[`third_party/wintun/PREBUILT-BINARIES-LICENSE.txt`](https://github.com/Iman/caspian/blob/main/third_party/wintun/PREBUILT-BINARIES-LICENSE.txt) ve şuraya kopyalanır:
Kurulum sırasında `C:\Program Files\Caspian\WINTUN-LICENSE.txt`.

`caspian-tethering.exe` ve `CaspianControl.exe` bağımsız .NET'tir
programlar. .NET bileşenleri yürütülebilir dosyaların içindedir, yanında değil
bunları ekstra DLL'ler olarak kullanın. .NET lisansı ve bildirimleri `third_party/dotnet/`'de bulunmaktadır.
Windows SDK referans paketi bir derleme girişidir ve Windows SDK ile yüklenmez.
Caspian.

Ayrıca `hostapd`, `dnsmasq`, `nftables`, `iw` ve `iproute2`'ye de ihtiyacı vardır.
makine. Bunlar bağlantılı olmak yerine ayrı programlar olarak çalışırlar, dolayısıyla
lisanslar bunu etkilemez, ancak onlar olmadan cihaz bir hiçtir.

[`NOTICE`](https://github.com/Iman/caspian/blob/main/NOTICE) tam kaydı taşır: ikili dosyadaki her modül, lisans okunur
kendi lisans dosyasından ve uyumluluk gerekçesinden.



<!-- SNI upstream credits -->

SNI kimlik sahtekarlığı kredileri: [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing) (GPL-3.0), Windows x64'te WinDivert (LGPL-3.0) ile.
[Üçüncü taraf lisanslar, kaynak sürümleri ve krediler](https://github.com/Iman/caspian/blob/feature/sni/docs/THIRD-PARTY.md).

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

<!-- Caspian guide navigation -->

Caspian kılavuzları: [kurulum ve desteklenen protokoller](https://github.com/Iman/caspian/wiki/Home.tr) · [DPI'yı aşmak için SNI sahtekarlığı: kurulum ve sınırlar](https://github.com/Iman/caspian/wiki/SNI-Spoofing.tr).

[WinDivert — Fesleğen (fesleğen00)](https://github.com/basil00/WinDivert/tree/v2.2.2): Windows x64, LGPL-3.0.


<!-- English-source-sha256: 55e2110d5bfd7485670033a177167abbb056f030356d7a78eb46204807b5452e -->
