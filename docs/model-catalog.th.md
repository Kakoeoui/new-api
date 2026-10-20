# คัดเฉพาะโมเดลจาก backup ไปหลังบ้านตัวเทส

คัดจาก `private/shop/shop-linux-move.sql` เป็น `private/shop/model-catalog.json` แล้ว
ได้โมเดลที่ไม่ถูกลบ 46 รายการ กับผู้ให้บริการที่อ้างถึง 3 ราย
ไฟล์นี้อยู่ในโฟลเดอร์ส่วนตัวที่ Git ignore

ข้อมูลที่นำเข้า: ชื่อ คำอธิบาย ไอคอน แท็ก รูปแบบ endpoint สถานะ กฎจับคู่ชื่อ
และข้อมูลผู้ให้บริการของโมเดล
ไม่คัดลอก users, tokens, channels, options, ประวัติใช้งาน, API keys หรือการตั้งราคา
โมเดลจะปรากฏในหน้าจัดการโมเดล แต่ยังไม่มีช่องทางสำหรับยิง API จริง
ตลาดโมเดลอาจแสดงว่าไม่พร้อมใช้งาน จนกว่าจะตั้งช่องทางของตัวเทสเอง

## ส่งไฟล์จาก Windows ไป VPS

ใช้ user/path ตามเครื่องที่แจ้งไว้ แทน `VPS_IP` ด้วย IP หรือ SSH hostname จริง:

```powershell
cd D:\codeforvps
scp .\private\shop\model-catalog.json zeal4eupa@VPS_IP:/home/zeal4eupa/rvl/private/shop/
scp .\new-api\scripts\model_catalog.py .\new-api\scripts\import-staging-models.sh zeal4eupa@VPS_IP:/home/zeal4eupa/rvl/test/new-api/scripts/
```

สองไฟล์ใน `scripts/` เป็น utility สำหรับนำเข้าข้อมูลครั้งเดียว ไม่ใช่ตัวรันเว็บเพิ่ม
จะ commit/push แล้ว pull scripts บน checkout ตัวเทสแทนการ SCP ก็ได้
ไฟล์ JSON ไม่ต้องขึ้น GitHub

## นำเข้า local

ถ้าต้องการโมเดลพร้อมราคาและช่องทาง API จาก production เพื่อให้แสดงในตลาดโมเดล:

```powershell
.\03-local.bat -ImportProductionModels
```

อ่านไฟล์ `../private/shop/shop-linux-move.sql` โดยไม่รัน SQL ในไฟล์
แทนที่ชุดโมเดล ผู้ให้บริการ ช่องทาง และ abilities ของ local พร้อมตั้งค่าราคา/กลุ่ม/สกุลเงินที่เกี่ยวข้อง
เก็บบัญชีลูกค้า ยอดเงิน ประวัติ และการตั้งค่าเว็บของ local ไว้ สำรองฐานก่อนทำใน volume เดิม
ไฟล์สำรองชื่อ `one-api.db.before-shop-เวลา.db` การเรียก API หลังนำเข้าจะใช้คีย์และเครดิตร้านจริงใน backup
ครั้งถัดไปใช้ `03-local.bat` ปกติ ไม่ต้องนำเข้าซ้ำ

ถ้าต้องการเฉพาะชื่อและรายละเอียดโมเดล โดยไม่คัดลอกคีย์หรือช่องทาง:

ใช้ตัวรันเดิมในโฟลเดอร์ `new-api`:

```powershell
.\03-local.bat -ImportModels
```

จะเริ่ม backend local แล้วนำเข้า `../private/shop/model-catalog.json` เข้า volume
`new-api-local-data` เท่านั้น พร้อมสำรองฐานก่อนนำเข้า จากนั้นเปิด frontend แบบ realtime ตามปกติ
ครั้งต่อไปเปิด `03-local.bat` ได้เลย โมเดลจะยังอยู่ ไม่ต้องนำเข้าซ้ำ
นำเข้าเฉพาะข้อมูลโมเดล 46 รายการกับผู้ให้บริการ 3 รายการ ไม่รวมลูกค้า ช่องทาง API หรือคีย์
โมเดลจะแสดงในหน้าจัดการโมเดล ส่วนการเรียกใช้งานต้องตั้งช่องทางทดสอบเอง

## นำเข้าบน VPS

เปิดตัวเทส `01-test.sh` ให้สร้าง backend และฐาน SQLite อย่างน้อยหนึ่งครั้งก่อน
จากนั้นเปิด SSH อีกหน้าต่างแล้วรัน:

```sh
cd /home/zeal4eupa/rvl/test/new-api
bash scripts/import-staging-models.sh /home/zeal4eupa/rvl/private/shop/model-catalog.json
```

utility นี้ทำงานกับ container `new-api-staging` และ volume `new-api-staging-data` เท่านั้น
ตรวจชื่อ image กับ volume ก่อนทำงาน และไม่รับชื่อ production เป็นปลายทาง
จะ pull Python image ก่อนพัก backend ตัวเทสชั่วคราว แล้วนำเข้าด้วย container ที่ปิด network
หลังทำงานจะเปิด backend ตัวเทสกลับ หากเดิมเปิดอยู่ โดยไม่หยุด production

- สำรอง SQLite ทั้งฐานก่อนนำเข้า ไฟล์สำรองอยู่ใน volume ชื่อ `one-api.db.before-models-เวลา.db`
- ใช้ transaction: ถ้าการ insert ล้มเหลวจะย้อนกลับรายการที่กำลังนำเข้าทั้งหมด
- ใช้ ID ของผู้ให้บริการในฐานเทส จึงไม่เอา ID จากฐานจริงมาชนกัน
- ถ้าชื่อโมเดลหรือผู้ให้บริการมีอยู่แล้ว จะใช้รายการเดิม ไม่ทับการแก้ไขของตัวเทส
- ผลลัพธ์รายงาน `models_added`, `models_skipped`, `vendors_added` และตำแหน่ง backup

เมื่อสำเร็จ รีเฟรชหลังบ้านหน้าโมเดลของตัวเทส
หากมีครบ 46 ชื่ออยู่แล้ว `models_added` อาจเป็น 0 ซึ่งเป็นการข้ามชื่อเดิมตามปกติ

## คัดจาก backup ชุดอื่นในอนาคต

```powershell
cd D:\codeforvps\new-api
python scripts/model_catalog.py export --dump ..\private\shop\shop-linux-move.sql --output ..\private\shop\model-catalog-new.json
```

ต้องมี Python 3 ในเครื่องสำหรับ export ตัวคัดอ่านเฉพาะ COPY ของ models/vendors เป็นข้อความ
ไม่รัน SQL และไม่ส่งทั้ง dump เข้า container ตัวเทส
เลือกชื่อ output ใหม่เพื่อเก็บ export เดิมไว้ แล้วส่งไฟล์ใหม่ไปนำเข้าด้วย utility เดิม

ทดสอบกับฐาน SQLite จำลองแล้ว ยังไม่ได้นำเข้าบน VPS จริงจากสภาพแวดล้อมนี้
