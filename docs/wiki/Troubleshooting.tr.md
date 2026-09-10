<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Troubleshooting) | [فارسی](https://github.com/Iman/caspian/wiki/Troubleshooting.fa) | [Русский](https://github.com/Iman/caspian/wiki/Troubleshooting.ru) | [中文](https://github.com/Iman/caspian/wiki/Troubleshooting.zh) | [العربية](https://github.com/Iman/caspian/wiki/Troubleshooting.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Troubleshooting.tr) | [اردو](https://github.com/Iman/caspian/wiki/Troubleshooting.ur)

</div>

<a id="troubleshooting-for-home-users"></a>
# Ev kullanıcıları için sorun giderme



Yönlendiricinizden Caspian çalıştıran bilgisayara Ethernet ile başlayın. Erişim noktası için bu bilgisayarın yerleşik Wi-Fi'sini veya Linux'ta uyumlu bir USB Wi-Fi adaptörünü kullanın. Bu, internet bağlantısı ve sıcak nokta için ayrı adaptörler sağlar. Bu, ölçülen bir hız garantisi değil, önerilen başlangıç düzenlemesidir.

<a id="wi-fi-country-is-not-set"></a>
## Wi-Fi ülkesi ayarlanmamış

Otomatik algılama varsayılan olarak kalır. Caspian tespit edemediği sürece Ülke alanını boş bırakın. "Wi-Fi ülkesi ayarlanmadı" mesajını görüyorsanız Wi-Fi ülkesini Gelişmiş ayarlara ayarla seçeneğini izleyin. Bilgisayarın bulunduğu ülkenin iki harfli kodunu girin, kaydedin ve Caspian'ı tekrar açın. Kaydedilen seçim, hizmetin yeniden başlatılmasından sonra hayatta kalır. Otomatik algılamaya dönmek için alanı temizleyin ve kaydedin. Caspian IR'yi üstlenmez veya panel dilinden veya proxy sunucusundan bir ülke seçmez.

Yüklü sürümünüz Ülke alanını gizliyorsa bu kurtarma kontrolü, 3 numaralı sorun güncellemesinin bir parçasıdır. Caspian ve Linux sürümlerinizi, adaptör düzenlemenizi ve güncellenen panelde Ülke seçeneğinin yeterli olup olmadığını kaydedin. Özel yapılandırma veya tam günlükler göndermeyin. Manuel bir iw komutunun yardımcı olduğuna dair bir rapor, uygulamanın sistem radyo ayarlarını değiştirmesi gerektiğini kanıtlamaz. Otomatik iw reg seti bu değişikliğin bir parçası değildir.


<a id="choose-your-connection"></a>
## Bağlantınızı seçin

Bu şemalarda, [1] internet yönlendiriciniz, [2] Caspian'ı çalıştıran bilgisayar ve [3] telefonunuz veya başka bir cihazdır. ETH Ethernet kablosu anlamına gelir. Bir USB Ethernet adaptörü interneti getirir; USB Wi-Fi adaptörü kablosuz bağlantı oluşturur. Farklı işler yapıyorlar.

```text
A  [1] --ETH--> [2] --built-in Wi-Fi--> [3]
B  [1] --ETH--> [2] --USB Wi-Fi-------> [3]
C  [1] --Wi-Fi A--> [2] --Wi-Fi B----> [3]
D  [1] --Wi-Fi--> [2: one radio] --Wi-Fi--> [3]
```

| Caspian'a internet | Cihazlarınızın erişim noktası | Linux / Ahududu Pi | macOS |
|---|---|---|---|
| A. Ethernet | Dahili Wi-Fi | Sürücü bir erişim noktası oluşturabildiğinde desteklenir | Desteklenen düzenleme |
| B. Ethernet | Harici USB Wi-Fi | Erişim noktası (AP) desteğine sahip bir Linux sürücüsü gerektirir | Caspian tarafından sıcak nokta olarak desteklenmiyor |
| C. Wi-Fi adaptörü A | Ayrı Wi-Fi adaptörü B | B bağdaştırıcısında AP desteği gerektirir | Harici bir USB Wi-Fi erişim noktası desteklenmiyor |
| D. Wi-Fi | Aynı Wi-Fi radyo | Koşullu: sürücünün bir istasyon ve AP'nin birlikte kullanılmasına izin vermesi gerekir; kanal paylaşılabilir | Dahili radyoda desteklenmiyor |

Bunlar mevcut kodun planlayabileceği veya reddedebileceği düzenlemelerdir. Her bağdaştırıcıyı, işletim sistemi güncellemesini veya dizüstü bilgisayarı onaylamazlar. Ev ağınıza katılabilecek bir Wi-Fi bağdaştırıcısı yine de bir erişim noktası oluşturamayabilir. Linux USB düzenlemeleri modellenmiş testlere sahiptir; mevcut donanım kaydı her USB adaptörünün çalıştığını kanıtlamaz. MacOS'ta belgelenen yol için Ethernet'i ve yerleşik Wi-Fi'yi kullanın. USB Wi-Fi'yi takmak bu kısıtlamayı ortadan kaldırmaz.

<a id="connect-first-then-start"></a>
## Önce bağlanın, sonra başlayın

1. Yalnızca erişim noktasına bağlı bir telefonda değil, Caspian'ı çalıştıran bilgisayarda çalışın. Erişim noktasını durdurmak telefonun bağlantısını keser.

2. Caspian'ı kapalı tutun. Ethernet kablosunu çalışan bir yönlendirici LAN bağlantı noktasına ve bilgisayara bağlayın. Uygulamayı açmadan önce herhangi bir USB Ethernet veya desteklenen USB Wi-Fi adaptörünü takın.

3. Bilgisayarın Ağ ayarlarını kontrol edin: Ethernet bağlı olmalıdır. Wi-Fi da bağlıysa, çalışan bir web sitesi tek başına kablonun internet taşıdığını kanıtlamaz. Ev Wi-Fi ağıyla bağlantıyı kesin ve tekrar kontrol edin; Wi-Fi radyosunu erişim noktası için kullanılabilir durumda tutun.

4. Caspian hâlâ kapalıyken normalde bu bağlantıyla çalışan bir web sitesi açın. Başarısız olursa önce kabloyu, yönlendirici bağlantısını veya ağda oturum açmayı düzeltin. Caspian'ın mevcut bir internet bağlantısına ihtiyacı var.

5. Caspian'ı ve web panelini açın. Panel hizmeti çalışıyorsa bilgisayarda http://127.0.0.1:8088/'yi kullanın. Bir telefondaki bu adres Caspian bilgisayarına değil, telefona atıfta bulunmaktadır.

6. Panelin internet bağlantısını ve hotspot adaptör seçeneklerini kontrol edin. Otomatik seçimle başlayın. Caspian yanlış bağlantıyı seçerse internet için Ethernet'i ve erişim noktası için amaçlanan Wi-Fi adaptörünü seçin. USB adaptörünün adı değişiklik gösterir; Başka birinin kılavuzundan arayüz adını kopyalamayın.

7. Proxy yapılandırmanızı ve erişim noktası adınızı/şifrenizi kaydedin. 5 GHz'i göremeyen cihazlar için 2,4 GHz bandıyla başlayın. Gerçek ülke kodunuzu kullanın ve değiştirmek için bir nedeniniz olmadığı sürece kanalı otomatik olarak bırakın.

8. Caspian'ı bir kez açın ve sonucu bekleyin. Caspian Control'ün Hazır demesi, hizmetlerinin yanıtı anlamına gelir; Tünel ve erişim noktası durumu için web panelini kontrol edin. Bir telefonda yeni Wi-Fi ağına katılın, ardından bir web sitesini test edin.

<a id="if-you-changed-a-cable-or-adapter"></a>
## Bir kabloyu veya adaptörü değiştirdiyseniz

Caspian'ı durdurun, bağlantıyı değiştirin ve yeniden başlamadan önce yukarıdaki internet kontrolünü tekrarlayın. Tarayıcıyı veya kontrol penceresini kapatmak arka plan hizmetlerinin mutlaka durdurulması anlamına gelmez. Tekrarlanan tıklamalar desteklenmeyen bir bağdaştırıcıyı onarmaz.

Mac'te Caspian Control'ü açın, Gelişmiş seçenekler'i seçin ve ardından Hizmetleri yeniden başlatın. Sonucu bekleyin, paneli tekrar açın ve kapalıysa Caspian'ı açın. Yeniden başlatma, bağlı cihazların kesintiye uğramasına ve panelin kapanmasına neden olabilir. Kaydedilmiş proxy ve sıcak nokta ayarlarını tutar.

Standart systemd yükleyicisiyle kurulan Linux'ta aşağıdaki komut her iki Caspian hizmetini de yeniden başlatır. Yerel bir terminal veya sıcak noktanın durdurulmasına dayanacak ayrı bir bağlantı kullanarak Caspian bilgisayarında çalıştırın. Bu, macOS, Windows veya systemd'siz bir kapsayıcı için bir komut değildir.

```bash
sudo systemctl restart caspian.service caspian-panel.service
```

Komut tamamlandıktan sonra paneli yeniden açın ve gerekirse Caspian'ı açın. Bir hizmet yine de başarısız olursa aşağıdaki rapor için hatayı saklayın. Tekrarlanan yeniden başlatmalardan kaçının; Hatanın ortadan kalkması için yapılandırmanızı silmeyin veya güvenlik duvarını devre dışı bırakmayın. Web panelinde Gelişmiş > Geri koy ve yeniden başlat seçeneği, kayıtlı ayarlarla ağı kurtarmayı dener; bu, hizmetleri yeniden başlatmaktan farklıdır ve cihazların bağlantısı kesilebilir.

<a id="find-the-symptom"></a>
## Belirtiyi bulun

| Ne görüyorsun | Sonraki kontrol edilecek şey |
|---|---|
| Caspian'a başlamadan önce internet yok | Başka bir kablo veya yönlendirici LAN bağlantı noktasını deneyin. Ethernet'in işletim sistemine bağlandığını doğrulayın. Caspian kapalıyken herhangi bir ağ oturum açma işlemini tamamlayın. |
| Tek Wi-Fi adaptörü zaten kullanımda | Linux'ta Ethernet veya ayrı bir AP özellikli adaptör kullanın. Tek radyolu Wi-Fi'dan Wi-Fi'ye sürücü desteği gerekir ve bir kanalı paylaşabilir. MacOS'ta yerleşik Wi-Fi için Ethernet'i kullanın. |
| Erişim noktası özellikli adaptör / adaptör eksik | Wi-Fi'ye katılmak AP desteğinin kanıtı değildir. Linux'ta bağdaştırıcının Linux sürücüsünü ve AP özelliğini kontrol edin. Mac'te harici bir USB Wi-Fi adaptörü Caspian'ın erişim noktası olamaz. |
| Bağdaştırıcı meşgul veya erişim noktası başlamıyor | Caspian'ı durdur. İnternet bağdaştırıcınızın bağlantısını kesmeden, amaçlanan erişim noktası bağdaştırıcısının diğer ağla olan bağlantısını kesin. Başlattığınız diğer etkin noktaları durdurun. Caspian kapalıyken rakip bir VPN olup olmadığını kontrol edin ve tekrar deneyin. |
| Telefon etkin noktayı göremiyor | Web panelinin erişim noktasının çalıştığını söylediğini doğrulayın. Yaklaşın, 2,4 GHz'i deneyin ve ülke ayarını kontrol edin. Sabitlenmiş bir kanal gelen Wi-Fi'yi takip eder; etkin nokta kanalını değiştirmek tek başına onu geçersiz kılamaz. |
| Telefon Wi-Fi'yi görüyor ancak katılamıyor | Panel şifresini değil, erişim noktası şifresini kullanın. Şifreyi yeniden adlandırdıktan veya değiştirdikten sonra, kayıtlı eski Wi-Fi girişini unutun ve tekrar katılın. Adres almaya devam ederse Caspian'ı bir kez yeniden başlatın ve tekrarlanırsa hatayı bildirin. |
| Telefon katıldı ancak sayfalar açılmıyor | Trafiğe izin vermek istiyorsanız Trafik kes seçeneğini işaretleyin ve devam ettirin. Tünel hatasını okuyun. Bilgisayarın tarih ve saatini kontrol edin. Okunabilir bir yapılandırma, kullanılamayan bir sunucuya işaret edebilir; sağlayıcınıza hala çalışıp çalışmadığını sorun. Yanlış bağlantının test edilmesini önlemek için telefonun mobil verileri geçici olarak kapalıyken test yapın. |
| Kontrolde Hazır, ancak web panelinde kırmızı | Hazır, arka plan hizmetlerinin yanıt verdiğini doğrular. Web paneli tüneli ve etkin noktayı bildirir. Tekrar tekrar yeniden yüklemek yerine tam mesajını kaydedin. |
| Panel durdurulduktan veya yeniden başlatıldıktan sonra kayboldu | Hizmetler çalışmaya başladıktan sonra Caspian bilgisayarından http://127.0.0.1:8088/ adresinden yeniden bağlanın. Erişim noktası durduğunda telefon panele olan yolunu kaybeder. Yerel ağ erişimi varsayılan olarak kapalıdır. |
| Uykudan sonra arıza, bağlantı istasyonunun fişinin çekilmesi veya ağlar arasında geçiş yapılması | Bilgisayarı uyandırın, kabloyu ve adaptörleri yeniden bağlayın, Caspian kapalıyken interneti doğrulayın ve yeniden başlatın. Diğer cihazlar erişim noktasına bağlıyken ana bilgisayarı uyanık tutun. |
| macOS uygulamayı engelliyor | MacOS kurulum kılavuzunu takip edin. Doğrulanmamış geliştirici uyarısı ve adlandırılmış kötü amaçlı yazılım algılaması farklı işlemler gerektirir. Trojan veya başka bir kötü amaçlı yazılımın adını veren uyarıyı atlamayın. |

<a id="check-the-configuration-format"></a>
## Yapılandırma formatını kontrol edin

Caspian, hy2 takma adı da dahil olmak üzere VLESS, VMess, Shadowsocks, SOCKS, Trojan ve Hysteria2 bağlantılarını kabul eder. Ayrıca desteklenen Clash/Clash.Meta YAML, Xray JSON, bağlantı listeleri ve base64 abonelik içeriğini de kabul eder. Seçtiğiniz listenin hangi girişini kullanır. Yapılandırmanın yanına bir abonelik adresi kaydedilebilir ve tünel aracılığıyla düğmeye bastığınızda yenilenebilir. Sağlayıcınızdan hesap şifresini veya web sayfası bağlantısını değil, desteklenen gerçek yapılandırmayı isteyin.

Desteklenen aktarım adları arasında raw/tcp, ws, grpc, httpupgrade, xhttp/splithttp ve kcp/mkcp bulunur. Protokol, aktarım ve güvenlik ayarları uyumlu olmalıdır; her kombinasyon işe yaramaz. TUIC, WireGuard, SSR, AnyTLS ve Hysteria v1 bağlantıları desteklenmez. Desteklenmeyen bir protokolün doğrulamayı geçmesini sağlamak için yeniden adlandırmayın. Kısıtlamalar ve test kanıtları için protokol kılavuzuna bakın.

<a id="ask-for-help-without-sharing-secrets"></a>
## Sırları paylaşmadan yardım isteyin

Hata raporu formunu kullanın ve bize şunları bildirin: Caspian sürümü, işletim sistemi/dağıtım ve sürüm, Ethernet-Wi-Fi veya Wi-Fi-to-Wi-Fi düzenlemesi, her adaptörün yerleşik mi yoksa harici mi olduğu, internetin başlamadan önce çalışıp çalışmadığı, tam hata ve daha önce denenmiş adımlar. Bağdaştırıcı yonga seti veya sürücü adını biliyorsanız faydalıdır; seri numarası eklemeyin.

Proxy bağlantıları, abonelik içerikleri, yapılandırma dosyaları, şifreler, anahtarlar, QR kodları, genel IP adresleri, ev Wi-Fi adları, MAC/BSSID adresleri, kişisel ana bilgisayar adları veya incelenmemiş günlükler/ekran görüntüleri göndermeyin. Kısa hatayı kopyalayın ve tanımlayıcıları kaldırın. Paneli internete maruz bırakmayın veya destek için bir yönlendirici bağlantı noktası iletmeyin.

<a id="known-limitations-and-evidence"></a>
## Bilinen sınırlamalar ve kanıtlar

Kusur kaydı, güvenlik ve kurtarma boşluklarını kaydeder. Bu sorun giderme adımları bunları kapatmaz. Örneğin, başka bir program tarafından kaldırılan bir güvenlik duvarı kural kümesini geri yükleyen periyodik bir kontrol yoktur. Aynı anda başka bir ağ paylaşım aracını çalıştırmaktan kaçının. Aşağıdaki topoloji testleri, kontrollü girişlerle planlamayı ve reddetmeleri doğrular; bunlar donanımınız üzerinde yeni bir test değildir.

- [Installation](https://github.com/Iman/caspian/wiki/Installation.tr)
- [Protokol ayrıntıları](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.tr)
- [Sorun bildir](https://github.com/Iman/caspian/issues/new?template=bug_report.yml)
- [Bilinen kusurlar](https://github.com/Iman/caspian/blob/main/docs/DEFECTS.md)
- [Kod ve test kanıtı](https://github.com/Iman/caspian/blob/main/internal/netcfg/plan_test.go)
- [macOS: Ethernet / Wi-Fi](https://support.apple.com/en-ie/guide/mac-help/mchlp1540/mac)



<!-- Caspian guide navigation -->

Caspian kılavuzları: [kurulum ve desteklenen protokoller](https://github.com/Iman/caspian/wiki/Home.tr) · [DPI'yı aşmak için SNI sahtekarlığı: kurulum ve sınırlar](https://github.com/Iman/caspian/wiki/SNI-Spoofing.tr).


<!-- English-source-sha256: d78e0c5791a008273b814aa9a9f9c4e6ba14c9875fd74ecd8a734c8e600f7ed4 -->
