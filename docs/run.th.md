# ตัวรันหลัก 3 ตัว

ถ้าต้องการนำเฉพาะรายการโมเดลจาก backup มาใช้ในตัวเทส ดู [วิธีนำเข้าโมเดล](model-catalog.th.md)

| ตัวรัน | ใช้ที่ไหน | หน้าที่ | ข้อมูล | อยู่ใน GitHub |
|---|---|---|---|---|
| `01-test.sh` | VPS Linux | แก้ frontend แบบ realtime | SQLite ทดสอบ แยกจากลูกค้า | ไม่อยู่ |
| `02-production.sh` | VPS Linux | Build และเปิดเว็บจริง | PostgreSQL ลูกค้าชุดเดิม | ไม่อยู่ |
| `03-local.bat` | เครื่อง Windows | แก้ frontend แบบ realtime | SQLite local แยกอีกชุด | อยู่ |

ตัวรันเก่าที่ชื่อ `run-dev`, `run-test`, `run-prod`, `run-customer`, `run-customer-dev`
ถูกนำออกแล้ว ตัวรัน VPS สองตัวเตรียมไว้ที่ `D:\codeforvps\private\vps`
ซึ่งอยู่นอก repo และถูก Git ignore ต้องส่งสองไฟล์นี้ไป VPS แยกครั้งแรก
โค้ดเว็บ/backend และไฟล์ Docker ยังขึ้น GitHub ตามเดิม

## 3 — ใช้บนเครื่องตัวเอง

ติดตั้ง Bun และเปิด Docker Desktop ในโหมด Linux containers จากนั้นดับเบิลคลิก
`D:\codeforvps\new-api\03-local.bat`

- หน้าเว็บ realtime: `http://127.0.0.1:5173`
- Backend local: `127.0.0.1:3001`
- ข้อมูลอยู่ใน volume `new-api-local-data`
- Image backend: `new-api:local`; container: `new-api-local`

ครั้งแรก build เฉพาะ Go backend ด้วย `Dockerfile.dev` แล้วเปิด dev server
ครั้งต่อไปใช้ image และข้อมูลเดิม ไม่ build Docker ซ้ำ
แก้ `web/src/` แล้วบันทึกเพื่อดูหน้าเปลี่ยนทันที
หากแก้ Go backend ค่อยรัน `03-local.bat -RebuildBackend`
ปิดหน้าต่าง dev เก่าด้วย Ctrl+C ก่อน หากพอร์ต 5173 ยังถูกใช้งาน
Ctrl+C หยุด frontend ส่วน backend Docker เก็บไว้ให้เปิดรอบถัดไปได้เร็ว

## เตรียม VPS ครั้งแรก

VPS ต้องมี Git, Docker Engine พร้อม Compose plugin และ Bun
ใช้บัญชีที่รัน Docker ได้ แทน `root@VPS_IP` ด้วยบัญชี SSH ของคุณ

แยก checkout ของตัวเทสออกจากตัวจริง:

```sh
mkdir -p /root/rvl/test /root/rvl/production
install -d -m 700 /root/rvl/private /root/rvl/private/shop /root/rvl/private/vps
git clone --branch test --single-branch https://github.com/Kakoeoui/new-api.git /root/rvl/test/new-api
git clone --branch test --single-branch https://github.com/Kakoeoui/new-api.git /root/rvl/production/new-api
```

branch ปัจจุบันคือ `test`; หากเปลี่ยนชื่อให้ปรับคำสั่งตามจริง
หาก repo เป็น private ต้องตั้ง GitHub deploy key ให้ VPS clone/pull ได้
ถ้ามี production checkout หรือฐานข้อมูลเดิมอยู่แล้ว ให้ใช้ของเดิม ไม่ต้อง clone ทับ

ส่งตัวรัน VPS จาก PowerShell บนเครื่องตัวเอง:

```powershell
cd D:\codeforvps
scp .\private\vps\01-test.sh .\private\vps\02-production.sh root@VPS_IP:/root/rvl/private/vps/
```

บน VPS:

```sh
chmod 700 /root/rvl/private/vps/01-test.sh /root/rvl/private/vps/02-production.sh
```

## 1 — เทสแบบ realtime บน VPS

```sh
/root/rvl/private/vps/01-test.sh
```

- ใช้โค้ดที่ `/root/rvl/test/new-api`
- Frontend: `127.0.0.1:5173`; backend: `127.0.0.1:3001` ของ VPS
- Image: `new-api:staging`; container: `new-api-staging`
- ข้อมูลทดสอบอยู่ใน volume `new-api-staging-data`
- แก้ React/CSS ใน checkout นี้แล้วบันทึก หน้าอัปเดตผ่าน HMR ไม่ต้อง build ใหม่
- รันใน terminal ค้างไว้; Ctrl+C หยุด frontend
- หากแก้ Go ใช้ `01-test.sh --rebuild-backend`

ดูเว็บผ่าน SSH tunnel จาก PowerShell บนเครื่องตัวเอง โดยปิด dev local ก่อนเพื่อไม่ให้ชนพอร์ต 5173:

```powershell
ssh -L 5173:127.0.0.1:5173 root@VPS_IP
```

เปิด `http://127.0.0.1:5173` บนเครื่องตัวเอง จะเห็นหน้าเทสที่รันบน VPS
ไม่ต้องเปิด dev server ออกอินเทอร์เน็ตโดยตรง
หากวาง staging checkout ที่อื่น ใช้ `RVL_STAGE_REPO=/path/to/repo /root/rvl/private/vps/01-test.sh`

## 2 — เว็บจริงบน VPS

ครั้งแรกส่ง `private/shop/shop-linux-move.sql` กับ `private/shop/.env.customer`
ไป `/root/rvl/private/shop` แยกผ่าน SCP โดยเก็บรหัสเดิมไว้:

```powershell
cd D:\codeforvps
scp .\private\shop\shop-linux-move.sql .\private\shop\.env.customer root@VPS_IP:/root/rvl/private/shop/
```

บน VPS:

```sh
chmod 600 /root/rvl/private/shop/shop-linux-move.sql /root/rvl/private/shop/.env.customer
cd /root/rvl/production/new-api
/root/rvl/private/vps/02-production.sh
```

ตัวรันใช้โฟลเดอร์ที่วาง script หากมี `docker-compose.customer.yml` อยู่ด้วย
หรือใช้โฟลเดอร์ปัจจุบันที่คุณสั่งรัน จึงไม่บังคับให้โค้ดอยู่ที่ `/root/rvl/production/new-api`
ยังใช้ Compose ชุด customer เดิม
หน้าเว็บกับ API อยู่ใน app container เดียวกัน พอร์ต `127.0.0.1:18474`
ให้ HTTPS reverse proxy ของเว็บจริงชี้มาพอร์ตนี้
PostgreSQL ไม่ publish พอร์ตออก host

ตัวรัน build ให้สำเร็จก่อนเริ่มหรืออัปเดตบริการ
SQL ถูก import เฉพาะ volume ใหม่ และ app รอ marker การ restore สำเร็จ
ข้อมูลเดิมยังใช้ volume `new-api-customer-postgres` และ `new-api-customer-app`
ห้ามลบ volume หรือเปลี่ยนรหัส DB ใน env โดยไม่เปลี่ยนใน PostgreSQL ด้วย

หาก checkout หรือข้อมูลอยู่คนละตำแหน่ง:

```sh
bash /root/rvl/private/vps/02-production.sh /path/to/new-api /path/to/private
```

หรือใช้ตัวแปรแวดล้อม:

```sh
RVL_PROD_REPO=/path/to/repo RVL_PRIVATE_DIR=/path/to/private /root/rvl/private/vps/02-production.sh
```

## อัปเดตโค้ดจาก GitHub

บนเครื่อง Windows อัปจาก `D:\codeforvps\new-api` เท่านั้น:

```powershell
cd D:\codeforvps\new-api
git add .
git diff --cached --stat
python scripts/check_github_files.py --history
git commit -m "Update RVL"
git push -u origin HEAD
```

ตรวจว่ารายการไม่มี SQL, env ที่มีรหัส หรือสองไฟล์ใน `private/vps`
ทดสอบโค้ดบน VPS ที่ staging checkout ก่อน เมื่อพร้อมจึงอัปเดต production:

```sh
cd /root/rvl/production/new-api
git pull --ff-only origin test
/root/rvl/private/vps/02-production.sh
```

การแก้ frontend ใน staging ไม่เปลี่ยนหน้าเว็บจริงจนกว่าจะนำโค้ดเดียวกันมา build ที่ production
รอบอัปเดต UI ไม่ต้องส่ง SQL ซ้ำ GitHub ไม่ได้เป็นที่เก็บฐานข้อมูล
สำรอง PostgreSQL จริงก่อนอัปเดต backend เพราะอาจมี migration

การจัดไฟล์นี้ไม่ได้หยุด container/ลบ volume เก่าที่มีอยู่ และไม่ได้ deploy ไป VPS ให้แล้ว
หากมีไฟล์อัปโหลดในระบบเดิม ต้องย้ายแยกจาก SQL ด้วย
