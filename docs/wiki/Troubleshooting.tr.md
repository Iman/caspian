# Ev kullanıcıları için sorun giderme

<div dir="ltr" align="left">

[English](https://github.com/Iman/caspian/wiki/Troubleshooting) | [فارسی](https://github.com/Iman/caspian/wiki/Troubleshooting.fa) | [Русский](https://github.com/Iman/caspian/wiki/Troubleshooting.ru) | [中文](https://github.com/Iman/caspian/wiki/Troubleshooting.zh) | [العربية](https://github.com/Iman/caspian/wiki/Troubleshooting.ar) | [اردو](https://github.com/Iman/caspian/wiki/Troubleshooting.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Troubleshooting.tr)

</div>

Önce yönlendiriciden Caspian çalıştıran bilgisayara Ethernet kablosu bağlayın. Erişim noktası için bilgisayarın dahili Wi-Fi adaptörünü veya Linux'ta uyumlu bir USB Wi-Fi adaptörünü kullanın. Böylece internet girişi ve erişim noktası ayrı adaptörlerde çalışır. Bu, önerilen başlangıç düzenidir; ölçülmüş bir hız garantisi değildir.

## Wi-Fi ülkesi ayarlanmamış

Otomatik algılama varsayılan olarak kalır. Caspian ülkeyi algılayabiliyorsa Country alanını boş bırakın. Wi-Fi country is not set hatasında Set Wi-Fi country bağlantısını açın. Gelişmiş ayarlarda bilgisayarın bulunduğu gerçek ülkenin iki harfli kodunu girin, kaydedin ve Caspian’ı yeniden açın. Seçim, hizmet yeniden başlatıldığında korunur. Otomatik algılamaya dönmek için alanı boşaltıp kaydedin. Caspian IR kodunu varsaymaz; panel dilinden veya proxy sunucusundan ülke seçmez.

Kurulu sürüm Country alanını gizliyorsa görünür alan issue #3 güncellemesinin parçasıdır. Caspian ve Linux sürümlerini, adaptör bağlantılarını ve güncel panelde ülke seçmenin sonucunu bildirin. Özel yapılandırma veya tam günlük göndermeyin. Elle çalıştırılan iw komutunun işe yaraması, uygulamanın sistem radyo ayarlarını değiştirmesi gerektiğini kanıtlamaz. Bu değişiklik otomatik iw reg set içermez.


## Bağlantı düzenini seçin

Şemalarda [1] internet yönlendiricisini, [2] Caspian bilgisayarını, [3] telefonu veya başka bir cihazı gösterir. ETH, Ethernet kablosudur. USB Ethernet adaptörü bilgisayara internet getirir; USB Wi-Fi adaptörü kablosuz bağlantı sağlar. Görevleri farklıdır.

```text
A  [1] --ETH--> [2] --built-in Wi-Fi--> [3]
B  [1] --ETH--> [2] --USB Wi-Fi-------> [3]
C  [1] --Wi-Fi A--> [2] --Wi-Fi B----> [3]
D  [1] --Wi-Fi--> [2: one radio] --Wi-Fi--> [3]
```

| Caspian'a gelen internet | Cihazlara sunulan erişim noktası | Linux / Raspberry Pi | macOS |
|---|---|---|---|
| A. Ethernet | Dahili Wi-Fi | Sürücü erişim noktası oluşturabiliyorsa desteklenir | Desteklenen düzen |
| B. Ethernet | Harici USB Wi-Fi | Erişim noktası (AP) destekleyen Linux sürücüsü gerekir | Caspian bunu erişim noktası olarak desteklemez |
| C. Wi-Fi adaptörü A | Ayrı Wi-Fi adaptörü B | B adaptöründe AP desteği gerekir | Harici USB Wi-Fi erişim noktası desteklenmez |
| D. Wi-Fi | Aynı Wi-Fi radyosu | Koşullu: sürücü istasyon ve AP rollerini birlikte desteklemelidir; kanal ortak olabilir | Dahili radyoda desteklenmez |

Tablo, mevcut kodun planlayabildiği veya reddettiği düzenleri gösterir. Her adaptörün, işletim sistemi güncellemesinin veya dizüstünün çalıştığını doğrulamaz. Ev Wi-Fi ağına katılabilen bir adaptör, erişim noktası oluşturamayabilir. Linux USB düzenlerinin model testleri vardır; mevcut donanım kayıtları tüm USB adaptörlerini doğrulamaz. Mac'te belgelenen yol Ethernet ve dahili Wi-Fi'dır. USB Wi-Fi eklemek bu kısıtı kaldırmaz.

## Önce bağlantıyı kurun, sonra başlatın

1. İşlemleri Caspian çalıştıran bilgisayarda yapın. Yalnızca erişim noktasına bağlı bir telefon kullanırsanız, erişim noktası durunca telefonun bağlantısı kesilir.

2. Caspian kapalı kalsın. Kabloyu yönlendiricinin çalışan bir LAN portuna ve bilgisayara takın. Uygulamayı açmadan önce USB Ethernet veya desteklenen USB Wi-Fi adaptörünü bağlayın.

3. Bilgisayarın ağ ayarlarında Ethernet'in bağlı olduğunu doğrulayın. Wi-Fi da bağlıysa, bir sitenin açılması kablodan internet geldiğini kanıtlamaz. Ev Wi-Fi ağından ayrılıp yeniden deneyin; Wi-Fi radyosu erişim noktası için kullanılabilir kalsın.

4. Caspian kapalıyken bu bağlantıda normalde açılan bir siteyi açın. Açılmıyorsa önce kabloyu, yönlendirici bağlantısını veya ağ oturum açma işlemini düzeltin. Caspian mevcut bir internet bağlantısına ihtiyaç duyar.

5. Caspian'ı ve web panelini açın. Panel hizmeti çalışıyorsa bilgisayarın kendisinde http://127.0.0.1:8088/ adresini kullanın. Telefonda bu adres Caspian bilgisayarını değil, telefonun kendisini gösterir.

6. Panelde internet bağlantısı ve erişim noktası adaptörü seçimlerini kontrol edin. Otomatik seçimle başlayın. Yanlış bağlantı seçilirse internet için Ethernet'i, erişim noktası için istediğiniz Wi-Fi adaptörünü seçin. USB arayüz adları değişir; başka bir bilgisayarın arayüz adını kopyalamayın.

7. Vekil sunucu yapılandırmasını, erişim noktası adını ve parolasını kaydedin. 5 GHz görmeyen cihazlar için 2.4 GHz ile başlayın. Bulunduğunuz ülkenin kodunu seçin; değiştirmek için bir nedeniniz yoksa kanal otomatik kalsın.

8. Caspian'ı bir kez açın ve sonucu bekleyin. Caspian Control'deki Ready, hizmetlerin yanıt verdiğini gösterir. Tünel ve erişim noktası durumunu web panelinden kontrol edin. Önce bir telefonu yeni ağa bağlayıp bir site deneyin.

## Kabloyu veya adaptörü değiştirdiyseniz

Caspian'ı durdurun, bağlantıyı değiştirin ve başlatmadan önce interneti yeniden kontrol edin. Tarayıcıyı veya kontrol penceresini kapatmak arka plan hizmetlerini mutlaka durdurmaz. Art arda tıklamak desteklenmeyen bir adaptörü düzeltmez.

Mac'te Caspian Control'ü açın, Advanced options > Restart services seçeneğini kullanın. Sonucu bekleyin, paneli yeniden açın ve Caspian kapalıysa açın. Yeniden başlatma bağlı cihazları keser ve paneli kapatabilir. Kaydedilmiş vekil sunucu ve erişim noktası ayarları korunur.

Standart systemd kurucusuyla kurulmuş Linux'ta aşağıdaki komut iki Caspian hizmetini yeniden başlatır. Komutu Caspian bilgisayarının yerel terminalinde veya erişim noktası durduğunda kesilmeyecek ayrı bir bağlantı üzerinden çalıştırın. Bu komut macOS, Windows veya systemd bulunmayan bir konteyner için değildir.

```bash
sudo systemctl restart caspian.service caspian-panel.service
```

Komut bitince paneli açın ve gerekirse Caspian'ı açın. Hizmet yine başarısız olursa hata metnini rapor için saklayın. Sürekli yeniden başlatmayın; hatayı gizlemek için yapılandırmayı silmeyin veya güvenlik duvarını kapatmayın. Panelde Advanced > Put it back and start again, kaydedilmiş ayarlarla ağı kurtarmayı dener. Bu, hizmetleri yeniden başlatmaktan farklıdır ve cihazların bağlantısı kesilebilir.

## Belirtiyi bulun

| Gördüğünüz durum | Sonraki kontrol |
|---|---|
| Caspian başlamadan önce de internet yok | Başka kablo veya yönlendirici LAN portu deneyin. İşletim sisteminde Ethernet bağlantısını doğrulayın. Gerekli ağ oturumunu Caspian kapalıyken açın. |
| Tek Wi-Fi adaptörü zaten kullanılıyor | Linux'ta Ethernet veya AP destekleyen ayrı adaptör kullanın. Tek radyoda iki rol sürücü desteği gerektirir ve aynı kanalı paylaşabilir. macOS'ta Ethernet'ten dahili Wi-Fi'a bağlantı kullanın. |
| Erişim noktası adaptörü yok veya görünmüyor | Wi-Fi'a bağlanabilmek AP desteğini kanıtlamaz. Linux'ta sürücüyü ve AP yeteneğini kontrol edin. Mac'te harici USB Wi-Fi, Caspian erişim noktası olamaz. |
| Adaptör meşgul veya erişim noktası başlamıyor | Caspian'ı durdurun. İnternet adaptörünü kesmeden erişim noktası adaptörünü diğer ağından ayırın. Açtığınız başka erişim noktalarını durdurun. Caspian kapalıyken başka VPN çalışıp çalışmadığını kontrol edip yeniden deneyin. |
| Telefon erişim noktasını görmüyor | Panelde çalıştığını doğrulayın. Bilgisayara yaklaşın, 2.4 GHz deneyin ve ülke ayarını kontrol edin. Sabitlenmiş kanal gelen Wi-Fi'ı izler; yalnızca erişim noktası kanalını değiştirmek bunu geçersiz kılamaz. |
| Telefon ağı görüyor ama bağlanamıyor | Panel parolasını değil, erişim noktası parolasını kullanın. Ad veya parola değiştiyse eski kayıtlı ağı unutup yeniden bağlanın. Adres alınıyor aşamasında kalırsa Caspian'ı bir kez yeniden başlatın; tekrarlanırsa hatayı bildirin. |
| Telefon bağlı ama sayfalar açılmıyor | Traffic cut durumunu kontrol edin; erişime izin vermek istiyorsanız trafiği sürdürün. Tünel hatasını, bilgisayarın tarih ve saatini kontrol edin. Okunabilen yapılandırmanın sunucusu erişilemez olabilir; sağlayıcıya sorun. Başka bağlantıyı test etmemek için telefonun mobil verisini geçici kapatın. |
| Control Ready diyor ama web paneli kırmızı | Ready, hizmetlerin yanıtını doğrular. Panel tüneli ve erişim noktasını gösterir. Tekrar tekrar kurmak yerine hata metnini kaydedin. |
| Durdurma veya yeniden başlatmadan sonra panel kayboldu | Hizmetler çalışınca Caspian bilgisayarında http://127.0.0.1:8088/ açın. Erişim noktası durunca telefonun panele giden yolu kaybolur. Yerel ağdan erişim varsayılan olarak kapalıdır. |
| Uyku, bağlantı istasyonunu çıkarma veya ağ değişiminden sonra hata | Bilgisayarı uyandırın, kabloyu ve adaptörleri bağlayın, Caspian kapalıyken interneti doğrulayın ve yeniden başlatın. Diğer cihazlar erişim noktasına ihtiyaç duyarken bilgisayarı uyanık tutun. |
| macOS uygulamayı engelliyor | Mac kurulum kılavuzunu izleyin. Doğrulanmamış geliştirici uyarısı ile belirli bir zararlı yazılım tespiti farklıdır. Truva atı veya başka zararlı yazılım adı veren uyarıyı aşmayın. |

## Yapılandırma biçimini kontrol edin

Caspian; VLESS, VMess, Shadowsocks, SOCKS, Trojan ve Hysteria2 bağlantılarını, hy2 takma adı dahil kabul eder. Desteklenen Clash/Clash.Meta YAML, Xray JSON, bağlantı listeleri ve base64 abonelik içeriğini de kabul eder. Listede ilk bağlantı kullanılır; abonelik URL'si indirilmez. Sağlayıcıdan hesap parolası veya web sayfası bağlantısı yerine desteklenen yapılandırmanın kendisini isteyin.

Desteklenen taşıma adları raw/tcp, ws, grpc, httpupgrade, xhttp/splithttp ve kcp/mkcp'dir. Protokol, taşıma ve güvenlik ayarları uyumlu olmalıdır; her birleşim çalışmaz. TUIC, WireGuard, SSR, AnyTLS ve Hysteria v1 bağlantıları desteklenmez. Doğrulamayı geçmek için desteklenmeyen protokolün adını değiştirmeyin. Kısıtlar ve test kanıtları protokol kılavuzundadır.

## Sırları paylaşmadan yardım isteyin

Hata formunda Caspian sürümünü, işletim sistemi veya Linux dağıtımını ve sürümünü, Ethernet-to-Wi-Fi ya da Wi-Fi-to-Wi-Fi düzenini, her adaptörün dahili veya harici olduğunu, başlamadan önce internetin çalışıp çalışmadığını, tam hatayı ve denediğiniz adımları yazın. Biliyorsanız yonga seti veya sürücü adı yararlıdır; seri numarası eklemeyin.

Vekil sunucu bağlantıları, abonelik içeriği, yapılandırma dosyaları, parolalar, anahtarlar, QR kodları, genel IP adresleri, ev Wi-Fi adları, MAC/BSSID, kişisel bilgisayar adları veya incelemediğiniz günlük ve ekran görüntülerini paylaşmayın. Kısa hatayı kopyalayıp tanımlayıcıları kaldırın. Destek için paneli internete açmayın veya yönlendiricide port yönlendirmeyin.

## Bilinen kısıtlar ve kanıtlar

Hata kaydı güvenlik ve kurtarma eksiklerini belgeler. Bu adımlar onları kapatmaz. Örneğin, başka bir programın kaldırdığı güvenlik duvarı kurallarını geri getiren düzenli kontrol yoktur. Aynı anda başka ağ paylaşım aracı çalıştırmayın. Aşağıdaki topoloji testleri kontrollü girdilerle planlamayı ve retleri doğrular; donanımınızda yapılmış yeni bir test değildir.

- [Kurulum](https://github.com/Iman/caspian/wiki/Installation.tr)
- [Protokol ayrıntıları (English)](https://github.com/Iman/caspian/wiki/Protocols-and-Transports)
- [Sorun bildir](https://github.com/Iman/caspian/issues/new?template=bug_report.yml)
- [Bilinen kusurlar](https://github.com/Iman/caspian/blob/main/docs/DEFECTS.md)
- [Kod ve test kanıtları](https://github.com/Iman/caspian/blob/main/internal/netcfg/plan_test.go)
- [macOS: Ethernet / Wi-Fi](https://support.apple.com/en-ie/guide/mac-help/mchlp1540/mac)

<div dir="ltr" align="left">

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md) | [العربية](https://github.com/Iman/caspian/wiki/Home.ar) | [اردو](https://github.com/Iman/caspian/wiki/Home.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Home.tr)

</div>
