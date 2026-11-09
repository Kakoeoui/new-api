# Handoff: ระบบล็อกอิน Discord, กู้คืนบัญชีด้วยคีย์ API, หน้าคีย์ API แบบ Minimal และระบบเช็คอินรายวัน (0.05 – 1.00 บาท)

เอกสารนี้สรุปรายละเอียดการอัปเดตระบบทั้งหมด ขั้นตอนการตั้งค่าบนเซิร์ฟเวอร์จริง (Production VPS) รายการไฟล์ที่แก้ไข และขั้นตอนการตรวจสอบหลังอัปเดต

---

## 1. ภาพรวมฟีเจอร์ที่อัปเดต

### 1.1 ระบบล็อกอิน/สมัครสมาชิกด้วย Discord อย่างเดียว + กู้คืนบัญชีเดิมด้วยคีย์ API (`แบบ ข.`)
- **ปิดระบบรหัสผ่านเดิมเป็นค่าเริ่มต้น**:
  - ตั้งค่า `PasswordLoginEnabled = false` และ `PasswordRegisterEnabled = false` ใน `common/constants.go`
  - ตั้งค่า `defaultDiscordSettings.Enabled = true` ใน `setting/system_setting/discord.go`
  - หน้า `/sign-in` และ `/sign-up` ซ่อนฟอร์ม Username/Password แบบเก่า และแสดงปุ่ม **เข้าสู่ระบบด้วย Discord** (`Continue with Discord`) / **สมัครสมาชิกด้วย Discord** (`Sign up with Discord`) เป็นปุ่มหลัก
- **ระบบกู้คืนและย้ายบัญชีด้วยคีย์ API คู่กับ Username (`POST /api/user/recover-by-key`)**:
  - ผู้ใช้กรอก **ชื่อผู้ใช้เดิม (Username)** คู่กับ **คีย์ API เดิม (`sk-...`)** (ใส่หรือไม่ใส่คำนำหน้า `sk-` ก็ได้)
  - การทำงานมี 2 กรณี:
    1. **กรณีล็อกอินด้วย Discord บัญชีใหม่ไว้แล้ว (ย้ายบัญชีมาผูก Discord ปัจจุบัน)**:
       - ระบบตรวจสอบว่า `username` และ `api_key` ตรงกับฐานข้อมูลและเป็นคีย์ของบัญชีนั้นจริง (เปรียบเทียบด้วย `subtle.ConstantTimeCompare`)
       - ย้าย `discord_id` จากบัญชีชั่วคราวที่เพิ่งล็อกอิน มาผูกเข้ากับบัญชีเดิม (`targetUser`)
       - โอนโควตา (`quota`) และคีย์ API (`tokens`) ทั้งหมดจากบัญชีชั่วคราวมารวมที่บัญชีเดิมโดยไม่สูญหาย
       - ปิดและลบ (`soft-delete`) บัญชีชั่วคราวที่ว่างเปล่าเพื่อไม่ให้ค้างซ้ำซ้อน
       - ออก Session ใหม่ของบัญชีเดิม (`setupLoginAtAuthVersion`) และสลับเข้าสู่บัญชีเดิมทันที
    2. **กรณีกดกู้คืนจากหน้าล็อกอินโดยตรง**:
       - หากกรอก `username` และ `api_key` ถูกต้อง ระบบจะเข้าสู่ระบบบัญชีเดิมให้ทันทีเพื่อให้ผู้ใช้สามารถกดผูก Discord ในหน้าตั้งค่าบัญชีต่อได้
  - มีปุ่มเปิดหน้าต่าง **`กู้คืน / ย้ายบัญชีเดิมด้วยคีย์ API`** ทั้งในหน้า `/sign-in`, `/sign-up` และแถบปุ่มด้านบนของหน้า `/keys` (`api-keys-primary-buttons.tsx`)

---

### 1.2 หน้าคีย์ API แบบ Minimal + หน้าต่างคำเตือนก่อนเปิดการตั้งค่าขั้นสูง
- **ตารางรายการคีย์ API (`web/src/features/keys/components/data-table-row-actions.tsx`)**:
  - นำปุ่มเปิด/ปิดคีย์ (ไอคอนวงกลมขีดฆ่า `Power`/`PowerOff`) ออกจากคอลัมน์การดำเนินการบนแถวตาราง เหลือเฉพาะปุ่ม **แก้ไข (Edit)** และเมนูเพิ่มเติม (`...`)
- **หน้าต่างสร้างและแก้ไขคีย์ API (`web/src/features/keys/components/api-keys-mutate-drawer.tsx`)**:
  - หมวด **ข้อมูลพื้นฐาน (`Basic Information`)** เหลือเฉพาะช่อง **ชื่อ (`Name`)** ช่องเดียวทั้งตอนสร้างคีย์ใหม่และตอนแก้ไขคีย์
  - ย้ายการตั้งค่าอื่นๆ ทั้งหมดลงไปอยู่ในหมวด **การตั้งค่าขั้นสูง (`Advanced Settings`)**:
    1. สวิตช์ **เปิดใช้งานคีย์ API (`Enable API Key`)** (แสดงเมื่อแก้ไขคีย์ เพื่อเปิด/ปิดสถานะคีย์แทนปุ่มหน้าตารางที่นำออก)
    2. **กลุ่ม / บ้าน (`Group`)** พร้อมลำดับ Auto Group และ Cross-group retry
    3. **เวลาหมดอายุ (`Expiration Time`)** และปุ่มลัด (`ไม่มีวันหมดอายุ`, `1M`, `1 วัน`, `1H`)
    4. **จำนวนคีย์ที่จะสร้าง (`Quantity`)** (แสดงเมื่อสร้างคีย์ใหม่)
    5. **การตั้งค่าโควต้า (`Quota Settings`)**: สวิตช์โควตาไม่จำกัด (`Unlimited Quota`) และช่องกรอกจำนวนโควตา
    6. **ขีดจำกัดโมเดล (`Model Limits`)**, **รายการ IP ที่อนุญาต (`IP Whitelist`)**, และ **อนุญาตให้คีย์ตรวจสอบโควต้าผู้ใช้ (`Allow querying user quota`)**
- **หน้าต่างคำเตือนก่อนเปิดการตั้งค่าขั้นสูง (`AlertDialog`)**:
  - เมื่อผู้ใช้กดคลิกที่หัวข้อ **การตั้งค่าขั้นสูง (`Advanced Settings`)** ตอนที่เมนูยังพับอยู่ ระบบจะยังไม่กางเมนูทันที แต่แสดงหน้าต่างคำเตือน:
    - ภาษาไทย: `"การตั้งค่าขั้นสูงเหล่านี้หากตั้งค่าผิดทางแอดมินไม่สามารถแก้ไขให้ลูกค้าได้"`
    - ภาษาอังกฤษ: `"If these advanced settings are configured incorrectly, the administrator will not be able to fix them for you."`
  - ปุ่มด้านล่างเรียงจากซ้ายไปขวา:
    - ปุ่มซ้าย: **`ออก` (`Exit`)** — ปุ่มสีเข้ม (`variant='default'`) กดแล้วปิดหน้าต่างเตือนโดยไม่กางการตั้งค่าขั้นสูง
    - ปุ่มขวา: **`ตกลง` (`OK`)** — ปุ่มสีอ่อน (`variant='outline'`) กดแล้วปิดหน้าต่างเตือนและกางการตั้งค่าขั้นสูง

---

### 1.3 ระบบเช็คอินรายวัน (เฉพาะบัญชีที่ล็อกอินด้วย Discord · สุ่ม 0.05 – 1.00 บาท)
- **อัตราแลกเปลี่ยนและค่าเริ่มต้น (`setting/operation_setting/checkin_setting.go`)**:
  - ระบบใช้หน่วย `QuotaPerUnit = 500,000` ต่อ `1.00 บาท` (`฿`)
  - ตั้งค่าเริ่มต้นของระบบเช็คอินรายวัน:
    - `Enabled: true` (เปิดใช้งานเป็นค่าเริ่มต้น)
    - `MinQuota: 25000` (`0.05 บาท`)
    - `MaxQuota: 500000` (`1.00 บาท`)
- **บังคับเฉพาะบัญชีที่เชื่อม/ล็อกอินด้วย Discord (`controller/checkin.go`, `model/checkin.go`)**:
  - `GET /api/user/checkin` คืนค่า `discord_linked` และ `require_discord: true`
  - `POST /api/user/checkin` และ `model.UserCheckin` ตรวจสอบ `model.IsUserDiscordLinked(userId)` (`users.discord_id != ""`) หากไม่ได้ผูกบัญชี Discord จะปฏิเสธการเช็คอินทันที
- **การแสดงผลบนหน้าเว็บ (`web/src/features/profile/components/checkin-calendar-card.tsx`)**:
  - แสดงการ์ดเช็คอินรายวันทั้งในหน้า **กระเป๋าเงิน (`/wallet`)** และหน้า **โปรไฟล์ (`/profile`)**
  - ปรับการแสดงผลตัวเลขสถิติเดือนนี้และสะสมทั้งหมดให้แสดงทศนิยม 2 ตำแหน่ง (`{ digitsLarge: 2, digitsSmall: 2 }`) เพื่อไม่ให้ปัดเศษสตางค์ทิ้ง
  - หากผู้ใช้ยังไม่ได้ล็อกอินด้วย Discord ปุ่มเช็คอินจะถูกล็อกพร้อมแสดงแถบแจ้งเตือนให้ใช้บัญชี Discord

---

### 1.4 ระบบภาษา (รองรับเฉพาะ ภาษาไทย และ English)
- ล็อกตัวเลือกภาษาใน `web/src/i18n/languages.ts` ให้แสดงเฉพาะ 2 ภาษา:
  - `ภาษาไทย` (`th`)
  - `English` (`en`)
- เพิ่มคีย์คำแปลใหม่ครบทั้งใน `web/src/i18n/locales/th.json` และ `web/src/i18n/locales/en.json` จำนวน 27 คีย์ รองรับการสลับภาษาแบบเรียลไทม์ทุกหน้าต่างและปุ่มที่เพิ่มใหม่

---

## 2. สิ่งที่ต้องตั้งค่าบนเซิร์ฟเวอร์จริง (Production Checklist)

หลังดึงโค้ดกิ่ง `aof` (หรือ merge เข้า `test`/`main`) ไปรันบน VPS แล้ว ให้ตรวจสอบ 3 จุดนี้ในระบบจริง:

### 2.1 ตั้งค่า Discord OAuth2 (หากยังไม่เคยตั้งค่าบนเว็บจริง)
1. เข้าไปที่ [Discord Developer Portal](https://discord.com/developers/applications) เลือกแอปของร้าน
2. ไปที่เมนู **OAuth2** -> **Redirects** แล้วเพิ่ม URL:
   ```text
   https://<โดเมนเว็บจริงของคุณ>/oauth/discord
   ```
   (เช่น `https://rvlconnect.com/oauth/discord` และตรวจสอบว่า `ServerAddress` ในหน้าตั้งค่าระบบตรงกับโดเมนนี้)
3. นำ **Client ID** และ **Client Secret** ไปกรอกในหน้า **ตั้งค่าระบบ (System Settings)** ของเว็บแอดมิน และเปิดใช้งาน Discord OAuth

### 2.2 ตรวจสอบค่า Option เดิมในฐานข้อมูล PostgreSQL (กรณีฐานข้อมูลเก่าเคยบันทึกค่าทับไว้)
เนื่องจาก New-API จะอ่านค่าในตาราง `options` ของฐานข้อมูลมาทับค่าเริ่มต้นในโค้ด หากฐานข้อมูลบน VPS เคยบันทึกค่าเก่าไว้ก่อนหน้านี้ แอดมินสามารถกดตั้งค่าผ่านหน้าเว็บแอดมิน หรือรัน SQL ด้านล่างในฐานข้อมูลจริงได้:

```sql
-- 1. เปิดระบบเช็คอินรายวัน และตั้งช่วงสุ่ม 0.05 - 1.00 บาท (25,000 - 500,000 quota)
INSERT INTO options ("key", "value") VALUES
  ('checkin_setting.enabled', 'true'),
  ('checkin_setting.min_quota', '25000'),
  ('checkin_setting.max_quota', '500000'),
  ('PasswordLoginEnabled', 'false'),
  ('PasswordRegisterEnabled', 'false'),
  ('discord.enabled', 'true')
ON CONFLICT ("key") DO UPDATE SET "value" = EXCLUDED."value";
```

### 2.3 คำสั่งอัปเดตบน VPS
```bash
# 1. อัปเดตโค้ดที่ staging/production ตามขั้นตอนปกติ
cd /root/rvl/production/new-api
git fetch origin
git checkout aof && git pull --ff-only origin aof

# 2. รันสคริปต์อัปเดต Production
/root/rvl/private/vps/02-production.sh
```

---

## 3. รายการไฟล์ที่สร้างและแก้ไขทั้งหมด

### Backend (Go)
- `controller/user_recovery.go` *(ใหม่)* — Endpoint `POST /api/user/recover-by-key` สำหรับตรวจสอบ Username + API Key, ย้ายการผูก `discord_id`, รวมโควตา/คีย์จากบัญชีชั่วคราว และออก Session ล็อกอิน
- `controller/checkin.go` — เพิ่มการตรวจสอบ `IsUserDiscordLinked` และส่งสถานะ `discord_linked`, `require_discord`
- `model/checkin.go` — เพิ่ม `IsUserDiscordLinked` และบังคับตรวจ `discord_id` ใน Transaction ก่อนบันทึกการเช็คอิน
- `setting/operation_setting/checkin_setting.go` — เปลี่ยนค่าเริ่มต้นเป็น `Enabled: true`, `MinQuota: 25000`, `MaxQuota: 500000`
- `controller/token.go` — รองรับการอัปเดต `status` (เปิด/ปิดคีย์) ไปพร้อมกับการบันทึกฟอร์มแก้ไขคีย์ (`UpdateToken`)
- `common/constants.go` — ตั้งค่าเริ่มต้น `PasswordLoginEnabled = false`, `PasswordRegisterEnabled = false`
- `setting/system_setting/discord.go` — ตั้งค่าเริ่มต้น `Enabled: true`
- `router/api-router.go` — ลงทะเบียน `POST /api/user/recover-by-key`
- `router/access_token_scope_test.go` — เพิ่ม `POST /api/user/recover-by-key` ใน `accessTokenExemptRoutes`

### Frontend (React / TypeScript)
- `web/src/features/auth/components/recover-by-key-dialog.tsx` *(ใหม่)* — หน้าต่างกู้คืน/ย้ายบัญชีเดิมด้วย Username + API Key
- `web/src/features/auth/components/oauth-providers.tsx` — แสดงปุ่ม Discord เป็นปุ่มหลักเสมอ
- `web/src/features/auth/hooks/use-oauth-login.ts` — รองรับการล็อกอินด้วย Discord และโหมดทดสอบบน Localhost
- `web/src/features/auth/sign-in/index.tsx` & `web/src/features/auth/sign-in/components/user-auth-form.tsx` — ปรับหน้าเข้าสู่ระบบให้เป็น Discord + ปุ่มกู้คืนบัญชีด้วยคีย์ API
- `web/src/features/auth/sign-up/index.tsx` — ปรับหน้าสมัครสมาชิกให้ใช้ Discord + ปุ่มกู้คืนบัญชีด้วยคีย์ API
- `web/src/features/keys/components/data-table-row-actions.tsx` — นำปุ่มเปิด/ปิดคีย์ออกจากแถวตาราง
- `web/src/features/keys/components/api-keys-mutate-drawer.tsx` — ปรับฟอร์มสร้าง/แก้ไขคีย์ให้ Minimal (เหลือเฉพาะช่องชื่อในหมวดพื้นฐาน ย้ายที่เหลือเข้าการตั้งค่าขั้นสูง พร้อมหน้าต่างเตือนก่อนเปิด)
- `web/src/features/keys/components/api-keys-primary-buttons.tsx` — เพิ่มปุ่มกู้คืนบัญชีด้วยคีย์ API ในหน้าจัดการคีย์
- `web/src/features/keys/lib/api-key-form.ts` & `web/src/features/keys/types.ts` — รองรับฟิลด์ `status` ในฟอร์มแก้ไขคีย์
- `web/src/features/profile/components/checkin-calendar-card.tsx`, `web/src/features/profile/index.tsx`, `web/src/features/profile/types.ts`, `web/src/features/wallet/index.tsx` — แสดงการ์ดเช็คอินรายวันในหน้ากระเป๋าเงินและโปรไฟล์ แสดงทศนิยม 2 ตำแหน่ง และบังคับเงื่อนไข Discord
- `web/src/i18n/languages.ts`, `web/src/i18n/locales/th.json`, `web/src/i18n/locales/en.json` — ล็อกตัวเลือกภาษาเหลือไทย/อังกฤษ และเพิ่มคำแปลทั้งสองภาษาครบทุกจุด
