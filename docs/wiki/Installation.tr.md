<!-- wiki-navigation:start -->
<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Installation) · [فارسی](https://github.com/Iman/caspian/wiki/Installation.fa) · [Русский](https://github.com/Iman/caspian/wiki/Installation.ru) · [简体中文](https://github.com/Iman/caspian/wiki/Installation.zh) · [العربية](https://github.com/Iman/caspian/wiki/Installation.ar) · [**Türkçe**](https://github.com/Iman/caspian/wiki/Installation.tr) · [اردو](https://github.com/Iman/caspian/wiki/Installation.ur)

</div>

<div dir="ltr" lang="tr">

[Caspian vikisi](https://github.com/Iman/caspian/wiki/Home.tr) · [Sorun giderme](https://github.com/Iman/caspian/wiki/Troubleshooting.tr)

</div>
<!-- wiki-navigation:end -->

<a id="installation"></a>
# Kurulum

[Bağlantı şemaları, ilk kablo kurulumu, hizmetin yeniden başlatılması ve yaygın hatalar için ev kullanıcısı sorun giderme kılavuzunu okuyun.](https://github.com/Iman/caspian/wiki/Troubleshooting.tr)

CPU ve RAM: Caspian'ın henüz ölçülen minimum RAM'i, CPU çekirdek sayısı veya saat hızı yok. Kaynak kullanımı trafik hacmine, proxy protokolüne ve eşzamanlı bağlantılara bağlıdır. Minimum gereksinimlerin yayınlanabilmesi için boşta kalma ve yük kıyaslamalarına ihtiyaç vardır.

Linux sürümü ikili dosyaları x86-64, ARM64 ve ARMv6/ARMv7'yi hedefler. Mimari uyumluluğu tek başına kullanılabilir performans sağlamaz.

> Bu kılavuz mevcut README'den alınmıştır. Ölçümleri orijinal tarihlerini koruyor; bu belgeleme hamlesi yeni bir test çalıştırmasını bildirmez.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="installing"></a>
## Kurulum

Önce işletim sistemini seçin. Windows ve macOS'ta grafik yükleyiciler bulunur;
Linux ve Raspberry Pi otomatik olarak kurulabilir, önce kontrol edilebilir veya oluşturulabilir
kaynaktan.

<a id="windows-10-and-11"></a>
### Windows 10 ve 11

Kurulum programı Caspian'ın ihtiyaç duyduğu her şeyi yükler. ihtiyacın yok
PowerShell, Go veya .NET SDK.

<a id="what-you-need"></a>
#### İhtiyacınız olan şey

- x64 veya ARM64 üzerinde Windows 10 sürüm 2004 (derleme 19041) veya üzerini ya da Windows 11 çalıştıran bir bilgisayar.
Yükleyici, Windows 10 sürüm 1607'den daha eski olan her şeyi reddeder;
ilk olarak Mobil Bağlantı Noktası ile. 1607 ve 1909 yılları arasında Caspian kurulumları ve panel
açılır, ancak sürümü belirten bir mesajla bağlantı reddedilir: bu yapılar
tünelden isim aramaları göndermenin hiçbir yolu yok ve Caspian içeri girmeyecek
adların ya sızdırıldığı ya da çözümlenmeyi bıraktığı bir durum.
- Bu bilgisayardaki bir yönetici hesabı.
- Windows Mobile Hotspot'u destekleyen bir Wi-Fi bağdaştırıcısı.
- Bir internet bağlantısı.
- Desteklenen bir proxy bağlantısı veya yapılandırması.

<a id="choose-the-correct-download"></a>
#### Doğru indirmeyi seçin

Çoğu Intel ve AMD bilgisayar x64 yükleyicisini kullanır:

- `CaspianSetup-0.2.1-windows-x64.exe`

Snapdragon veya başka bir ARM işlemciye sahip Windows bilgisayarlar ARM64'ü kullanır
yükleyici:

- `CaspianSetup-0.2.1-windows-arm64.exe`

Bilgisayarınızın türünü bilmiyorsanız **Ayarlar**'ı açın. **Sistem**'i seçin,
ardından **Hakkında**. **Sistem türü** satırını okuyun.

<a id="install-caspian"></a>
#### Caspian'ı yükleyin

1. [Caspian yayın sayfası](https://github.com/Iman/caspian/releases/latest)'yi açın.
2. En yeni sürümün altında **Varlıklar**'ı genişletin.
3. Doğru Windows yükleyicisini indirin.
4. İndirilen dosyaya çift tıklayın.
5. SmartScreen görünürse **Daha fazla bilgi** seçeneğini seçin.
6. Yayıncı uyarısının indirdiğiniz dosyaya ad verdiğinden emin olun.
7. **Yine de çalıştır**'ı seçin.
8. Windows yönetici erişimi istediğinde **Evet**'i seçin.
9. Lisans sayfasını okuyun ve devam edin.
10. Caspian web paneli için bir şifre seçin.
11. Aynı şifreyi tekrar yazın.
12. Bu şifreyi güvenli bir yerde saklayın.

Kurulum sihirbazı ayrıca iki isteğe bağlı seçeneği gösterir:

- **Masaüstü kısayolu oluşturun**
- **Oturum açtığımda Caspian Control'ü başlat**

Her iki seçenek de varsayılan olarak kapalıdır. Kurulum her zaman bir **Caspian Denetimi** oluşturur
Windows Başlat menüsündeki kısayol. Kurulum tamamlandığında **Açık Caspian'ı bırakın
Kontrol** seçilip **Son**'a tıklayın.

Yükleyici bir bildirim alana kadar Windows **Bilinmeyen yayımcı** uyarısı gösterebilir.
kod imzalama sertifikası. Dosyanın resmi Caspian'dan gelip gelmediğini kontrol edin
Devam etmeden önce sayfayı yayınlayın.

![Caspian Control on Windows](https://github.com/Iman/caspian/blob/main/docs/images/caspian-control-windows.png)

<a id="the-two-caspian-windows"></a>
#### İki Caspian penceresi

Caspian'ın Windows'ta iki farklı kontrol ekranı vardır.

| Ekran | Nerede açılıyor | Neyi kontrol ediyor |
|---|---|---|
| **Caspian Kontrolü** | Küçük bir Windows uygulaması ve bildirim alanı simgesi | Caspian arka plan hizmetlerini başlatır, durdurur veya yeniden başlatır |
| **Caspian web paneli** | `http://127.0.0.1:8088/` adresindeki web tarayıcınız | Wi-Fi adını, Wi-Fi şifresini, frekans bandını ve proxy bağlantısını ayarlar |

Önce **Caspian Control**'ü kullanın. **Hazır** seçeneğini bekleyin ve ardından **Paneli aç** seçeneğini seçin.
Web paneli ikinci ekrandır. Sıcak noktayı ve tüneli başlatmak için bunu kullanın.

Caspian Control'de **Hazır**, iki arka plan hizmetinin yanıt verdiği anlamına gelir.
Bu, proxy tünelinin bağlı olduğu anlamına gelmez. Web paneli yeşile döner
sıcak nokta ve tünel hazır olduğunda.

<a id="first-start"></a>
#### İlk başlangıç

1. Windows Başlat menüsünden veya masaüstünden **Caspian Control**'ü açın.
2. Windows yönetici erişimi istediğinde **Evet**'i seçin.
3. **Tümünü başlat**'ı seçin.
4. Büyük kartta **Hazır** yazana kadar bekleyin.
5. **Paneli aç** seçeneğini seçin.
6. Kurulum sırasında seçtiğiniz panel şifresini yazın.
7. **Oturum aç**'ı seçin.
8. Yeni Wi-Fi ağı için bir ad girin.
9. En az sekiz karakterden oluşan bir Wi-Fi şifresi girin.
10. Eski cihazlarda en iyi destek için **2,4 GHz**'i koruyun.
11. Proxy bağlantınızı veya yapılandırmanızı yapıştırın.
12. Caspian'ı başlatmak için anahtarı seçin.
13. Web paneli durumu yeşile dönene kadar bekleyin.
14. Telefonunuzu veya diğer cihazınızı yeni Wi-Fi ağına bağlayın.
15. Bağlantıyı test etmek için o cihazda bir web sitesi açın.

Panel bağlı her cihazı gösterir. Windows bu aygıtların adreslerini verir
`192.168.137.0/24`'den. Caspian internet trafiğini şu adresten gönderir:
`xray0` tüneli.

Panel şifresi ile Wi-Fi şifresi farklı. Panel şifresi açılır
web paneli. Wi-Fi şifresi telefonları ve diğer cihazları birbirine bağlar.

<a id="what-the-caspian-control-buttons-do"></a>
#### Caspian Control düğmeleri ne işe yarar?

| Kontrol | Sonuç |
|---|---|
| **Hepsini başlat** | Her iki Caspian arka plan hizmetini başlatır |
| **Hepsini durdur** | Her iki hizmeti de durdurur ve durdurulmasını sağlar |
| **Tümünü yeniden başlat** | Her iki hizmeti de durdurur ve başlatır |
| **Paneli aç** | Caspian web panelini tarayıcınızda açar |

Uygulama, penceresini kapattıktan sonra Windows bildirim alanında kalır.
Bildirim alanı saatin yanındadır. Caspian simgesini çift tıklayın
uygulamayı tekrar açın.

<a id="what-to-expect"></a>
#### Ne beklenebilir?

Caspian ağ rotalarını değiştirdiği için Windows yönetici erişimi istiyor.
güvenlik duvarı, Mobil Erişim Noktası ve Wintun ağ bağdaştırıcısı.

Erişim noktasını durdurduğunuzda veya yeniden başlattığınızda Windows aygıtların bağlantısını keser. Bekle
web panelinin yeşile dönmesini sağlayın, ardından her cihazı tekrar bağlayın.

Caspian Control **Hazır** diyorsa ancak web paneli kırmızıysa, içindeki mesajı okuyun.
web paneli. Web paneli, sıcak noktayı ve proxy tünelini test eder.

<a id="developer-requirements"></a>
#### Geliştirici gereksinimleri

Aşağıdaki PowerShell yöntemi geliştiriciler içindir. Caspian'ı bundan inşa ediyor
depoyu açar ve Windows hizmetlerini yükler.

Bu yöntem şu ek programlara ihtiyaç duyar:

- Bir yönetici hesabı.
- Aktif bir internet bağlantısı.
- Windows Mobile Hotspot'u destekleyen bir Wi-Fi bağdaştırıcısı.
- [Windows için Git](https://git-scm.com/download/win).
- [1.26 veya sonraki bir sürüme geçin](https://go.dev/dl/).
- [.NET 9 SDK'sı](https://dotnet.microsoft.com/download/dotnet/9.0).

Kurulum programı ve geliştirici yöntemi x64 ve ARM64 Windows sistemlerini destekler.

<a id="developer-install"></a>
#### Geliştirici kurulumu

1. PowerShell'i açın.
2. Bu depoyu klonlayın.
3. Depo dizinine geçin.
4. Windows yükleyicisini çalıştırın.

```powershell
git clone https://github.com/Iman/caspian.git
Set-Location caspian
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\packaging\windows\install.ps1
```

5. Windows yönetici erişimi istediğinde **Evet**'i tıklayın.
6. Derleme ve hizmet kurulumunun bitmesini bekleyin.

Yükleyici şu görevleri yerine getirir:

- Geçerli bilgisayar için `caspian.exe` derler.
- Windows Mobile Erişim Noktası yardımcısını oluşturur.
- `CaspianControl.exe` tepsi uygulamasını oluşturur.
- `wintun.dll` olmadığında Wintun 0.14.1'i indirir.
- Wintun arşivini sabit SHA-256 değeriyle karşılaştırır.
- Programları `C:\Program Files\Caspian`'ye yükler.
- `caspian` ve `caspian-panel` Windows hizmetlerini oluşturur.
- Her iki hizmeti de otomatik olarak başlayacak şekilde ayarlar.
- **Caspian Control** masaüstü kısayolu oluşturur.
- Yerel panel için 45 saniyeye kadar bekler.
- Panel cevap verdikten sonra Caspian Control'ü açar.

Tepsi uygulamasını açmadan yüklemek için `-NoOpen`'yi kullanın:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\packaging\windows\install.ps1 -NoOpen
```

<a id="repair-or-update"></a>
#### Onarın veya güncelleyin

Aynı yükleyiciyi tekrar çalıştırın. Yükleyici hizmetleri durdurur,
programlar, panel durumunu korur ve hizmetleri yeniden başlatır.

<a id="uninstall"></a>
#### Kaldır

Depoda bir yönetici PowerShell açın. Sonra çalıştırın:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\packaging\windows\uninstall.ps1
```

Kaldırıcı, Windows hizmetlerini ve yüklü programları kaldırır. Okuyun
Yerel durumu korumanız gerektiğinde kullanmadan önce komut dosyasını kullanın.

<a id="macos-13-or-later"></a>
### macOS 13 veya üzeri

MacOS disk görüntüsü yerel **Caspian Control** uygulamasını ve
Caspian motoru. Terminal, Go, Homebrew veya başka bir çalışma zamanına ihtiyacınız yok.
Bir yönetici hesabına ihtiyacınız vardır ve yerleşik Wi-Fi erişim noktası olduğunda,
kablolu bir Ethernet internet bağlantısı.

<a id="choose-the-correct-download-1"></a>
#### Doğru indirmeyi seçin

- Intel Mac'ler `Caspian-v0.2.4-macos-amd64.dmg` kullanır.
- Apple Silicon Mac'ler (M1 veya üstü) `Caspian-v0.2.4-macos-arm64.dmg` kullanır.

Mac'in bir Apple menüsü olup olmadığını bilmiyorsanız **Apple menüsü → Bu Mac Hakkında**'yı açın.
Intel işlemci veya Apple silikon.

<a id="install-and-approve-the-first-opening"></a>
#### İlk açılışı kurun ve onaylayın

v0.2.4 uygulaması geçici olarak imzalanmıştır ancak henüz bir Apple Geliştiricisi ile imzalanmamıştır
Apple tarafından kimlik veya noter tasdikli. Bu nedenle Gatekeeper **“Caspian” Açılmamış** gösteriyor
ve Apple'ın kötü amaçlı yazılım içermediğini doğrulayamadığını söylüyor. Bu bir şey değil
uygulama çökmesi. Uyarıyı yalnızca şuradan indirilen bir dosya için geçersiz kılın:
resmi Caspian yayın sayfası.

1. [Caspian'ın son sürümü](https://github.com/Iman/caspian/releases/latest)'yi açın
ve **Varlıklar**'ı genişletin.
2. Mac'in işlemcisi için DMG'yi indirin ve açın.
3. `Caspian.app`'yi **Uygulamalar** klasörüne sürükleyin.
4. Kopyayı **Uygulamalar**'da bir kez açın.
5. Gatekeeper bunu engellediğinde **Bitti**'ye tıklayın.
6. **Apple menüsü → Sistem Ayarları → Gizlilik ve Güvenlik**'i açın.
7. **Güvenlik** seçeneğine ilerleyin ve Caspian'ın yanındaki **Yine de Aç** seçeneğini tıklayın. Düğme
engellenen açma girişiminden sonra yaklaşık bir saat boyunca kullanılabilir durumda kalır.
8. Mac oturum açma parolasını girin, **Tamam**'a tıklayın ve **Aç**'ı onaylayın.

macOS bu uygulamayı bir istisna olarak kaydeder, bu nedenle daha sonraki açılışlar çift tıklanarak çalışır
normalde. Apple aynı süreci şu belgede belgeliyor:
[Güvenlik ayarlarını geçersiz kılarak bir uygulamayı açın](https://support.apple.com/guide/mac-help/apple-cant-check-app-for-malicious-software-mchleab3a043/26/mac/26).

<a id="if-macos-still-blocks-the-background-service"></a>
#### macOS arka plan hizmetini hâlâ engelliyorsa

Yüklenen arka planda yürütülebilir dosya `/usr/local/bin/caspian`, bir
`Caspian.app`'yi onayladıktan sonra karantina bayrağı. Uyarı adları küçük harf
`caspian` ve kontrol penceresi **Caspian'ın ilgilenilmesi gerekiyor** raporunu verebilir.

**Uyarı Trojan adını veriyorsa veya kötü amaçlı yazılım bildiriyorsa aşağıdaki komutu kullanmayın.**
Kurulumu durdurun ve tam uyarıyı, algılama adını, sürüm sürümünü ve
[GitHub sorunu](https://github.com/Iman/caspian/issues)'deki indirme URL'si.
Kötü amaçlı yazılım tespitinin araştırılması gerekir; imzasız bir yayın tek başına yeterli değildir
tespitin yanlış olduğunu tespit edin. Bkz.
[Apple'ın macOS güvenlik uyarılarına ilişkin açıklaması](https://support.apple.com/en-ie/102445).

Bu geri dönüşü yalnızca doğrulanmamış geliştirici veya onaylanmamış uygulama uyarısı için kullanın,
dosyaya ve kaynağına güvendikten sonra. Resmi yetkiliden yayını indirin
Caspian sürüm sayfasını açın ve DMG sağlama toplamını yayınlananlarla karşılaştırın
`SHA256SUMS`. Eşleşen bir sağlama toplamı, sürüm dosyasının güvenliğini değil, sürüm dosyasını doğrular.

1. **Terminal**'i açın.
2. Karantina bayrağını yüklü arka planda yürütülebilir dosyadan kaldırın:

   ```bash
   sudo xattr -d com.apple.quarantine /usr/local/bin/caspian
   ```

3. Mac oturum açma parolanızı girin. Terminal siz yazarken şifreyi görüntülemiyor.
4. Caspian'da **Gelişmiş seçenekler → Hizmetleri yeniden başlat**'ı seçin.

Bu komut yalnızca adlandırılmış dosyanın karantina özelliğini kaldırır. öyle değil
Yürütülebilir dosyayı tarayın, imzalayın veya noter tasdiki yapın. Terminal `No such xattr` rapor ederse,
özellik zaten yok. Hizmet yine de başarısız olursa hatayı bildirin
diğer güvenlik kontrollerini kaldırmak yerine.

<a id="let-caspian-set-itself-up-and-save-its-password"></a>
#### Caspian'ın kendisini kurmasına ve şifresini kaydetmesine izin verin

1. **Caspian Control**'ü başlatın. Paketlenmiş arka plan hizmetini aşağıdakilerle karşılaştırır:
Kurulu olan paneli kontrol etmeden önce.
2. İlk başlatmada veya DMG bir güncelleme içerdiğinde kurulum başlar
otomatik olarak. MacOS yetkilendirmesinde yönetici şifresini girin
diyalog. Kurulu sürüm zaten eşleştiğinde şifre istenmez.
3. Kontrol penceresinde **Caspian hazır** yazana kadar bekleyin. Yetkilendirme ise
iptal edilirse **Caspian'ı Kur** veya **Caspian'ı Güncelle** yeniden deneme için görünür durumda kalır.
4. İlk kurulumda, aşağıdaki tabloda gösterilen **ilk çalıştırma paneli şifresini** kaydedin.
çıktı. **Panel şifresini kopyala** yalnızca bu şifreyi kopyalar.
5. **Paneli aç**'a tıklayın ve kayıtlı panel şifresiyle oturum açın.
6. Wi-Fi adını ve şifresini girin, proxy yapılandırmasını yapıştırın ve ardından şunu kullanın:
Caspian'ı başlatmak için panel anahtarı.

Mac oturum açma parolası, Caspian panel parolası ve Wi-Fi parolası üçtür
farklı şifreler. Panel şifresi kaybolursa, **Şifreyi sıfırla** seçeneğini kullanın.
Caspian Kontrolü; yönetici yetkilendirmesi gerekli, ancak kaydedilen proxy
ve sıcak nokta ayarları kalır. Kontrol penceresi kapatıldığında menü çubuğundan çıkılır
çalışan öğe; yeniden açmak için **Caspian Denetimini Aç**'ı seçin.

MacOS'ta kontrol penceresini kapatmak Caspian'ı menü çubuğunda tutar. Seçim
**Caspian'dan çıkın ve hizmetleri durdurun**, sıcak nokta ve arka plan hizmetlerini durdurur.
macOS yetkilendirmesi iptal edilirse veya durdurma başarısız olursa uygulama açık kalır.
Bir sonraki başlatmada Caspian, durdurulan hizmetleri yönetici yetkisiyle bir kez başlatır.

<a id="linux-and-raspberry-pi"></a>
### Linux ve Raspberry Pi

<a id="automated-one-line"></a>
#### Otomatik: tek satır

sudo /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"

Yükleyici hangi makinede olduğunu bulur ve eşleşen ikili dosyayı indirir
en son sürümden itibaren ve indirmenin eşleşmiyor olması durumunda reddeder
sağlama toplamı yayınlandı.

| `uname -m` | eser | tipik makine |
|---|---|---|
| `x86_64` | `caspian-linux-amd64` | bir dizüstü bilgisayar veya mini PC |
| `aarch64` | `caspian-linux-arm64` | 64 bit sistemde Raspberry Pi 3, 4, 5 |
| `armv7l` | `caspian-linux-arm` | 32 bit sistemde Raspberry Pi 2 ve 3 |
| `armv6l` | `caspian-linux-arm` | Raspberry Pi 1, Sıfır, Sıfır W |

Emin olamadığında tahmin etmek yerine reddeder. Linux değil,
mimari bu tabloda yok, sistem bilgisi yok veya eşleşmeyen bir sağlama toplamı:
her biri bulduğu şeyi adlandırmayı reddediyor. `armv8l`, 64 bit üzerinde 32 bit kullanıcı alanı
çekirdek kasıtlı olarak eşlenmemiştir, çünkü önceki bir işlemin nasıl olduğunu tahmin ediyoruz.
proje ARMv7 kodunu ARMv6 makinelerine gönderdi ve onları ölüme terk etti
İlk çalıştırmada yasa dışı talimat.

Komut dosyasını bir kabuğa aktarmadan önce okuyun. Bu tavsiye formalite değil
bu tür yazılımlar için ve komut dosyası okunmak için yazılmıştır.

bukle -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh | daha az

Yukarıdaki komut betiği görüntüler; Caspian'ı yüklemez veya güncellemez.
Yükseltmek için kurulum komutunu tekrar çalıştırın. Yükleyici en son sürümü seçer
yayınlanan sürüm ve kayıtlı ayarlarınızı korur. Portal,
ikili. `main` üzerindeki değişiklikler, bunları içeren bir sürümden sonra görünür.

Belirli bir sürümü yüklemek için aşağıdaki örnek etiketi değiştirin:

sudo env CASPIAN_VERSION=v0.2.5 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"

<a id="forgotten-panel-password"></a>
### Unutulan panel şifresi

Caspian bilgisayarında bu komutu bir terminalde veya SSH oturumunda çalıştırın:

```bash
sudo /usr/local/bin/caspian reset-password
```

Komut, yeni bir panel parolası yazdırır ve paneli yeniden başlatır. Vekiliniz ve
Wi-Fi ayarları kayıtlı kalır. Giriş sayfasında yeni şifreyi kullanın. Windows'ta,
`& "$env:ProgramFiles\Caspian\caspian.exe" reset-password`'yi yöneticide çalıştırın
PowerShell penceresi. Caspian'ı yeniden yüklemek şifrenizi sıfırlamaz.

<a id="verifying-a-download-yourself"></a>
### Bir indirme işlemini kendiniz doğrulama

Her sürüm bir `SHA256SUMS` dosyası taşır. Yükleyici bunu sizin için kontrol eder ve
bağımsız olarak kontrol edebilirsiniz:

kıvrılma -fsSLO https://github.com/Iman/caspian/releases/latest/download/caspian-linux-arm64
kıvrılma -fsSLO https://github.com/Iman/caspian/releases/latest/download/SHA256SUMS
sha256sum -c SHA256SUMS --yoksay-eksik

Bu neyi kanıtlar ve neyi kanıtlamaz: sahip olduğunuz dosyanın dosya olduğunu kanıtlar
yayın yayınlandı. Bu sürümü kimin oluşturduğunu kanıtlamaz. İkili dosyalar
GitHub Actions tarafından etiketli bir taahhütten ve bunu oluşturan iş akışından oluşturulmuştur
bunlar [`.github/workflows/release.yml`](https://github.com/Iman/caspian/blob/main/.github/workflows/release.yml) adresindeki bu depodadır, dolayısıyla yapı şu şekildedir:
bağımsız olarak tekrarlanamasa bile okunabilir.

<a id="manual-build-it-yourself"></a>
#### Kılavuz: kendiniz oluşturun

Otomatik rota hakkında hiçbir şeye gerek yoktur. Kaynaktan inşa etmek için Go'ya ihtiyaç var
1.26 veya üstü ve işlev açısından ikili özdeşlik verir.

git klonu https://github.com/Iman/caspian.git
CD Caspian
git build -trimpath -o caspian ./cmd/caspian
sudo CASPIAN_LOCAL_BINARY="$PWD/caspian" bash install.sh

`CASPIAN_LOCAL_BINARY`, yükleyiciye yeni oluşturduğunuz dosyayı kullanmasını söyler.
birini indirmektense. Yükleyicinin hizmeti oluşturmak için yaptığı diğer her şey
hesap, dizinler, birimler ve bunların izinleri aynı şekilde gerçekleşir.

Başka bir makineden Pi için çapraz derleme:

GOOS=linux GOARCH=arm64 go build -trimpath -o caspian-linux-arm64 ./cmd/caspian
GOOS=linux GOARCH=arm GOARM=6 go build -trimpath -o caspian-linux-arm ./cmd/caspian

32 bit yapıdaki `GOARM=6` isteğe bağlı değildir. Hem `armv6l` hem de `armv7l`
makineler aynı `arm` yapısını kurar, böylece bir ARMv7 yapısı her Pi 1'i bozar,
Onu yükleyen Sıfır ve Sıfır W. Sürüm iş akışı bunu şununla kontrol eder:
`readelf` ve onun hakkında yalan söyleyen bir eseri yayınlamak yerine başarısız oluyor
Mimarlık.

Bir yapıya güvenmeden önce kapıyı çalıştırın:

bash betikleri/gate.sh

Paket başına formatlama, veterinerlik ve yarış dedektörü ile tüm paketi çalıştırır.
kapsama katları, altın regresyon katmanı, gizlilik taraması ve duman
altküme. Başarısızlık durumunda sıfır dışında çıkar. Hiçbir yere borulamayın: kabuk boru hattı
son komutunun durumunu rapor eder, bu nedenle onu `tail`'ye aktarmak gereksizdir
aradığınız cevap.

<!-- SNI upstream credits -->

SNI kimlik sahtekarlığı kredileri: Windows x64'te WinDivert (LGPL-3.0) ile [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing) (GPL-3.0).
[Üçüncü taraf lisanslar, kaynak sürümleri ve krediler](https://github.com/Iman/caspian/blob/feature/sni/docs/THIRD-PARTY.md).

<!-- English-source-sha256: abb023f6408ff919116ca296c4d914e430ac0fb2b6aea1caf73990f480c1c5d9 -->
