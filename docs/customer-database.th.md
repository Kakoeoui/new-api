# ฐานข้อมูลลูกค้า

[ตัวรันปัจจุบัน การส่งไฟล์ส่วนตัว และการเชื่อมฐานข้อมูล](run.th.md)

เว็บจริงใช้ `docker-compose.customer.yml` กับ volume เดิม `new-api-customer-postgres` และ `new-api-customer-app`
ไฟล์สำรอง PostgreSQL 15 และ `.env.customer` เก็บแยกจาก GitHub
นำเข้า backup เฉพาะ volume ใหม่ การเปิดซ้ำคงข้อมูลปัจจุบัน
ตัวแก้เว็บ realtime ทั้ง local และ VPS ใช้ SQLite แยกชุดจากฐานลูกค้า
