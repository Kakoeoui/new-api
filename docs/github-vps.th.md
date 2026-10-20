# แยก GitHub ออกจากข้อมูลร้าน

ขึ้น GitHub เฉพาะ repository `new-api` ทั้งชุด โค้ด backend ก็ต้องอยู่ด้วยเพื่อให้ build API ได้
ข้อมูลร้านจริงแยกอยู่นอก repository:

```text
D:\codeforvps\
├── new-api\                   ← โฟลเดอร์เดียวที่ใช้ Git / ขึ้น GitHub
│   ├── web\                   ← UI, ธีม, รูปและฟอนต์
│   ├── controller\, model\, relay\, ...  ← โค้ด API ไม่ใช่ฐานข้อมูลร้าน
│   ├── scripts\               ← ตัวรันและเครื่องมือนำเข้า ไม่มีข้อมูลร้านฝังอยู่
│   ├── .githooks\             ← ตรวจไฟล์ก่อน commit / push
│   ├── docs\
│   ├── Dockerfile, Dockerfile.dev, docker-compose.customer.yml
│   ├── .env*.example          ← ตัวอย่างเปล่าเท่านั้น
│   └── 03-local.bat
├── private\                   ← ห้ามขึ้น GitHub ทุกไฟล์
│   ├── shop\
│   │   ├── shop-linux-move.sql
│   │   ├── model-catalog.json
│   │   ├── .env.customer
│   │   └── .env.perf           ← สร้างเมื่อมีข้อมูล uptime
│   └── vps\
│       ├── 01-test.sh
│       └── 02-production.sh
├── frontend\, feature\        ← ต้นฉบับอ้างอิง ไม่ต้องอัป
└── .tools\, .bun-cache\       ← เครื่องมือในเครื่อง ไม่ต้องอัป
```

ไม่ต้องแยกย้าย `web/` กับ Go ออกจากกัน เพราะ Docker build ต้องใช้ทั้งสองส่วน
ข้อมูลบัญชีลูกค้า ยอดเงิน ประวัติ ช่องทาง API ราคา และ key ที่นำเข้า อยู่ในฐานข้อมูลหรือไฟล์ส่วนตัว ไม่อยู่ในซอร์สโค้ด
Docker volumes ไม่ได้ถูก Git อัปโหลด แต่ห้าม export volume หรือสำรองฐานข้อมูลมาใส่ repository
โค้ดและค่าหน้าเว็บที่ตั้งใจแสดงต่อผู้ใช้ เช่น โลโก้และ URL คู่มือ จะอยู่ใน GitHub

## เปิด local

```powershell
cd D:\codeforvps\new-api
.\03-local.bat
# เมื่อต้องการนำเข้าโมเดล ราคา และช่องทางจาก backup อีกครั้งเท่านั้น:
.\03-local.bat -ImportProductionModels
```

ตัวรันอ่านไฟล์ส่วนตัวจาก `../private/shop` อัตโนมัติ ต้องการเก็บที่อื่นให้ตั้ง
`$env:RVL_PRIVATE_DIR = 'D:\path\to\private\shop'` ก่อนรัน
เปิด frontend ตาม URL ที่ตัวรันแสดง; ข้อมูล Docker เดิมยังใช้ได้

## อัป GitHub

เปิด PowerShell ใน `new-api` เท่านั้น ใช้ Python 3 สำหรับตัวตรวจไฟล์:

```powershell
cd D:\codeforvps\new-api
git config core.hooksPath .githooks
python scripts/check_github_files.py --history
# ต้องขึ้น PASS ก่อนดำเนินการต่อ
git add .
git diff --cached --stat
python scripts/check_github_files.py --history
git commit -m "Update RVL website and deployment layout"
git push -u origin HEAD
```

เครื่องนี้เปิด hooks แล้ว; หลัง clone ใหม่ให้ตั้ง `core.hooksPath` ซ้ำ
ตัวตรวจบล็อกพาธข้อมูลส่วนตัว, SQL/DB, env จริง และตรวจเทียบรหัสที่พบใน backup/env ส่วนตัวบนเครื่อง
ตรวจทั้งไฟล์ใน working tree กับเวอร์ชันที่ staged; pre-push ตรวจชื่อไฟล์ในประวัติ Git เพิ่ม
มีข้อยกเว้น SQL migration ของโปรแกรมสองไฟล์ใน `bin/` ซึ่งเป็นโค้ด schema ไม่มีข้อมูลร้าน
ประวัติ upstream มี `web/.env` หนึ่งเวอร์ชันซึ่งมีตัวแปร frontend สองตัวเป็นค่าว่าง ตัวตรวจอนุญาตเฉพาะเนื้อหาว่างชุดนั้น
ตัวตรวจไม่ได้รับประกันตรวจพบ secret ทุกชนิดหรือข้อมูลที่เปลี่ยนรูปแบบแล้ว
หากเคยเผยแพร่รหัสจริงมาก่อน การเพิ่ม ignore ไม่ลบข้อมูลในประวัติ ต้องเปลี่ยนรหัสและจัดการประวัติแยกต่างหาก

ห้ามลาก `private/` อัปโหลดผ่านหน้าเว็บ GitHub และห้ามใช้ `git add -f` หรือข้าม hooks เพื่ออัปข้อมูลร้าน
ก่อน commit ให้ตรวจรายการ staged ทุกครั้ง; คำสั่งข้างต้นไม่ได้สั่งอัปโดยอัตโนมัติ

## แบ่งโฟลเดอร์บน VPS Linux

ใช้ `$HOME` ของบัญชีที่ SSH เข้า เช่น `/root` หรือ `/home/zeal4eupa`:

```text
$HOME/rvl/
├── production/new-api/    ← clone โค้ดสำหรับรันจริง
├── test/new-api/          ← clone อีกชุดสำหรับแก้ realtime
└── private/              ← ส่งแยก ไม่ใช้ Git
    ├── shop/             ← SQL, .env.customer และไฟล์ส่วนตัว
    └── vps/              ← 01-test.sh, 02-production.sh
```

ต้องมีสอง checkout เพื่อให้แก้ตัวเทสได้โดยไม่แก้ไฟล์ production
แต่ละตัวใช้ container/volume แยกกัน คู่มือเต็มดู [ตัวรัน 3 ตัว](run.th.md)

บน VPS เตรียมโฟลเดอร์ โดยไม่ clone ทับ checkout ที่มีอยู่แล้ว:

```bash
mkdir -p "$HOME/rvl/production" "$HOME/rvl/test"
install -d -m 700 "$HOME/rvl/private" "$HOME/rvl/private/shop" "$HOME/rvl/private/vps"
git clone --branch test --single-branch https://github.com/Kakoeoui/new-api.git "$HOME/rvl/test/new-api"
git clone --branch test --single-branch https://github.com/Kakoeoui/new-api.git "$HOME/rvl/production/new-api"
```

ส่งไฟล์ส่วนตัวผ่าน SSH/SCP แยกจาก GitHub ตัวอย่างบัญชี `zeal4eupa` (เปลี่ยน VPS_IP ก่อน):

```powershell
cd D:\codeforvps
scp .\private\vps\01-test.sh .\private\vps\02-production.sh zeal4eupa@VPS_IP:/home/zeal4eupa/rvl/private/vps/
scp .\private\shop\shop-linux-move.sql .\private\shop\.env.customer zeal4eupa@VPS_IP:/home/zeal4eupa/rvl/private/shop/
```

บน VPS:

```bash
chmod 700 "$HOME/rvl/private/vps/01-test.sh" "$HOME/rvl/private/vps/02-production.sh"
chmod 600 "$HOME/rvl/private/shop/shop-linux-move.sql" "$HOME/rvl/private/shop/.env.customer"
# เว็บจริง: build frontend + backend และใช้ฐานข้อมูลจริง
bash "$HOME/rvl/private/vps/02-production.sh" "$HOME/rvl/production/new-api" "$HOME/rvl/private/shop"
# ตัวเทส realtime: ใช้ terminal อีกอัน
bash "$HOME/rvl/private/vps/01-test.sh"
```

หาก production อยู่พาธอื่น ให้ส่งพาธจริงเป็น argument แรก; อย่าย้ายฐานข้อมูลหรือ volume ตามโค้ด
เว็บจริงใช้ `127.0.0.1:18474` ผ่าน HTTPS reverse proxy
ตัวเทสใช้ `127.0.0.1:5173` และ backend `127.0.0.1:3001`; ดูผ่าน SSH tunnel
SQL ใช้ restore เฉพาะ volume ใหม่ การอัปเดตโค้ดครั้งถัดไปไม่ต้องส่ง SQL ซ้ำ
ห้ามลบ volumes หรือเปลี่ยนรหัสฐานข้อมูลเดิมโดยไม่ได้เปลี่ยนใน PostgreSQL ด้วย
