<!-- wiki-navigation:start -->
<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Development-and-Testing) · [فارسی](https://github.com/Iman/caspian/wiki/Development-and-Testing.fa) · [Русский](https://github.com/Iman/caspian/wiki/Development-and-Testing.ru) · [简体中文](https://github.com/Iman/caspian/wiki/Development-and-Testing.zh) · [العربية](https://github.com/Iman/caspian/wiki/Development-and-Testing.ar) · [**Türkçe**](https://github.com/Iman/caspian/wiki/Development-and-Testing.tr) · [اردو](https://github.com/Iman/caspian/wiki/Development-and-Testing.ur)

</div>

<div dir="ltr" lang="tr">

[Caspian vikisi](https://github.com/Iman/caspian/wiki/Home.tr) · [Sorun giderme](https://github.com/Iman/caspian/wiki/Troubleshooting.tr)

</div>
<!-- wiki-navigation:end -->

<a id="development-and-testing"></a>
# Geliştirme ve test

> Bu kılavuz mevcut README'den alınmıştır. Ölçümleri orijinal tarihlerini koruyor; bu belgeleme hamlesi yeni bir test çalıştırmasını bildirmez.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="running-it"></a>
## Çalıştırmak

İkili dosyayı oluşturun ve yükleyiciye verin. Bu yolun serbest bırakılmasına gerek yok ve
yükleyici bunu gerçek bir kurulumun yanı sıra bir deneme çalışması için de alır:

git build -o /tmp/caspian-linux-arm64 ./cmd/caspian
sha256sum /tmp/caspian-linux-arm64 | sed 's|/tmp/||' > /tmp/SHA256SUMS

env CASPIAN_LOCAL_BINARY=/tmp/caspian-linux-arm64 \
CASPIAN_LOCAL_CHECKSUMS=/tmp/SHA256SUMS \
bash install.sh --dry-run --yes

Gerçek kurulum için `--dry-run`'yi bırakın. `CASPIAN_LOCAL_CHECKSUMS` olmadan
yükleyici bu sözlerle doğrulanmamış bir ikili dosya yüklediği konusunda uyarıyor.
[`docs/INSTALL.md`](https://github.com/Iman/caspian/blob/main/docs/INSTALL.md) tam runbook'tur. Sahte bir `uname` koşum takımı içerir
Kurulamayan bir makine üzerinde retlerin yürümesi.

İkili dosyanın dört alt komutu vardır:

caspian service --ayrıcalıklı kök: yollar, güvenlik duvarı, erişim noktası, motor
caspian service --panel caspian kullanıcısını: web paneli, ayrıcalıklı bir şey yok
caspian check bu kutunun neye benzediğini bildiriyor; hiçbir şeyi değiştirmez
Caspian versiyonu

Bir yapılandırmayı uygulayan veya anahtarı yönlendiren hiçbir alt komut kasıtlı olarak yoktur.
CLI'nin kendisi şunu söylüyor: "Yükleyici çalıştırıldıktan sonra kişinin yaptığı her şey
panelde olur."

[`uninstall.sh`](https://github.com/Iman/caspian/blob/main/uninstall.sh) birimleri, ikili dosyaları ve dizinleri kaldırır ve tekrar oynatır
ağ günlüğü, böylece kutu bulunduğu gibi bırakılır. Güvenmeden önce [kusur D5](https://github.com/Iman/caspian/wiki/Troubleshooting.tr)'yi okuyun.

<a id="the-rules-this-project-holds-itself-to"></a>
## Bu projenin uyması gereken kurallar

Bunlar arzu değil. Her birinin bir mekanizması vardır ve mekanizmanın adı da vardır.

**Gerçek trafikten alınan bir çıkış IP'si olmadan hiçbir şeye çalışma denemez.**
[`docs/2026-08-29-design.md`](https://github.com/Iman/caspian/blob/main/docs/2026-08-29-design.md), bölüm 6. Bağlantı bir sonuç değildir. Donanım
Hiçbir çıkış IP'si yakalanmadığında ve 1'den çıktığı zaman, emniyet kemeri notları BAŞARILI değil, KANITLANMAMIŞTIR.

**Kendinden emin, yanlış bir cümle, hiç cümle olmamasından daha kötüdür.**
bir şeyin doğru bir şekilde ele alınması, kontrol edilecek bir şeyin olmadığı sonucuna varır. Yani bir
düzeltme, daha iyi bir cümle yerine, geride bir test bırakır.
`TestNothingInTheApplianceWatchesTheUplink` var çünkü iki belge aynı anda
Kutunun yukarı bağlantıyı izlediğini ve hareket ettiğinde güvenlik duvarını yeniden yüklediğini iddia etti.

**Başlatılmış bir süreç, işe yaradığının kanıtı değildir.** Etkin nokta arayüzü
Herhangi bir şey ona bağlanmadan önce çekirdekten geri okunur ve erişim noktası
hizmetin çalıştığını bildirmeden önce tekrar okuyun. Her iki okuma da eklendi
Her komutun başarı ile sonuçlandığı ölçülü bir olaydan sonra.

**Her senaryonun başarısız olduğu görülmüştür.** `TestEveryScenarioCanFail` bir
her davranışta kusur olarak adlandırılır ve kırmızıya dönmesini gerektirir. Kimsenin yapmadığı bir test
başarısızlığın görülmesi, hiçbir şeye bağlı olmayan yeşil bir ışıktır.

**Bir fikstürün kaynağı dosya adındadır.** `capture-pi5-` bayttır
hedefe gerçek bir komut çıktısı veren `scenario-`, kimsenin sahip olmadığı bir makinedir
ölçülmüş ve `golden-` bu projenin kendi çıktısıdır. Bir test okuması
`capture-pi5-` dosyası hedef hakkında iddiada bulunuyor. `scenario-` değerini okuyan bir test
dosya yok.

**Taahhütteki kimlik bilgisi kalıcıdır.** `test/goldenscan` her şeyi tarar
kayıtlı nöbetçiler ve kimlik bilgileri şekilleri için kararlı bir fikstür ve
dosya adlarının yanı sıra dosya gövdelerini de kontrol eder. Dikilen bir bitkiyi yakalarken izlendi
Bildiği her sınıfın sırrı.

**Kapsama tabanları çok dişlidir.** [`scripts/gate.sh`](https://github.com/Iman/caspian/blob/main/scripts/gate.sh)'deki her sayı,
Birini hedef almak değil, onu tanıtan çalışma sonrasında ölçülen bir paket
umuyordum. Sırası olmayan bir paket geçitli değildir ve sıranın olmaması şu anlama gelir:
"Bu paket kapsam dahilindedir" yerine "henüz bir karara varılmadı".

**Ayrıcalıklı taraf arayanın gönderdiği hiçbir şeye güvenmez.** Her alanın her alanı
istek, bu makinenin kendisi için algıladıklarıyla karşılaştırılarak kontrol edilir. Bir ret bir
kapalı bir kümeden gelen hata kodu, asla bir cümle ve asla arayan kişinin değeri
gönderildi.

**Kutu internetten sizin istemediğiniz hiçbir şeyi istemez.** Telemetri yok, eve telefon yok, kilitlenme yok
varsayılan olarak yükleme yok, web yazı tipi yok, coğrafi veri dosyası yok ve Google çözümleyici yok.

[Architecture](https://github.com/Iman/caspian/wiki/Architecture.tr) | [Panel-and-Configuration](https://github.com/Iman/caspian/wiki/Panel-and-Configuration.tr) | [Troubleshooting](https://github.com/Iman/caspian/wiki/Troubleshooting.tr)

<!-- English-source-sha256: 5badcd2d45aa3aa7f927a0215bc4511dcd899932a2c648c8bec2484f9c77a161 -->
