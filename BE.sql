CREATE TABLE "alamat_pemesanan" (
    "id" SERIAL PRIMARY KEY, -- ID unik untuk alamat pemesanan
    "nama_penerima" VARCHAR(255) NOT NULL, -- Nama penerima barang
    "no_telp" VARCHAR(15) NOT NULL, -- Nomor telepon penerima
    "street" VARCHAR(255) NOT NULL, -- Jalan atau detail alamat
    "city" VARCHAR(255) NOT NULL, -- Kota
    "state" VARCHAR(255) NOT NULL, -- Provinsi
    "postal_code" VARCHAR(10) NOT NULL, -- Kode pos
    "country" VARCHAR(50) NOT NULL, -- Negara
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP -- Tanggal pembuatan alamat
);
