#!/usr/bin/env python3
"""
Script tạo test data: CSV với user data và nén bằng gzip
"""

import csv
import gzip
import random
import sys
from datetime import datetime

# Configuration
NUM_ROWS = 100000
OUTPUT_FILE = "users_2024-04-02.csv.gz"

# Sample data
FIRST_NAMES = [
    "Nguyen",
    "Tran",
    "Le",
    "Pham",
    "Hoang",
    "Huynh",
    "Phan",
    "Vu",
    "Vo",
    "Dang",
]
LAST_NAMES = ["Van", "Thi", "Minh", "Anh", "Hung", "Hoa", "Lan", "Nhan", "Tuan", "Lam"]
DOMAINS = ["gmail.com", "yahoo.com", "hotmail.com", "outlook.com"]
MESSAGES = [
    "Chào mừng tham gia chương trình đặc biệt!",
    "Đặc quyền mới dành cho bạn, hãy khám phá ngay!",
    "Ưu đãi giới hạn, không thể bỏ lỡ!",
    "Cảm ơn bạn đã đồng hành cùng chúng tôi!",
    "Sản phẩm mới vừa ra mắt, mời trải nghiệm!",
    "Giảm giá 50% cho đơn hàng đầu tiên!",
    "Quà tặng đặc biệt dành riêng cho bạn!",
    "Cập nhật tính năng mới, hãy thử ngay!",
    "Mời tham gia sự kiện đặc biệt cuối tuần!",
    "Tin vui: Bạn đã đủ điều kiện nhận quà!",
]


def generate_user_id(index):
    """Generate user ID starting from 1001"""
    return str(1000 + index)


def generate_email(user_id):
    """Generate email address"""
    domain = random.choice(DOMAINS)
    return f"user{user_id}@{domain}"


def generate_name():
    """Generate Vietnamese name"""
    first = random.choice(FIRST_NAMES)
    last = random.choice(LAST_NAMES)
    return f"{first} {last}"


def generate_phone(user_id):
    """Generate phone number"""
    return f"09{random.randint(10, 99)}{random.randint(100000, 999999)}"


def generate_message():
    """Generate random message"""
    return random.choice(MESSAGES)


def generate_csv():
    """Generate CSV file with user data"""
    print(f"Generating {NUM_ROWS} rows of data...")

    with gzip.open(OUTPUT_FILE, "wt", encoding="utf-8") as gz_file:
        writer = csv.writer(gz_file)

        # Write header
        writer.writerow(["user_id", "email", "name", "phone", "message"])

        # Write data rows
        for i in range(1, NUM_ROWS + 1):
            user_id = generate_user_id(i)
            email = generate_email(user_id)
            name = generate_name()
            phone = generate_phone(user_id)
            message = generate_message()

            writer.writerow([user_id, email, name, phone, message])

            if i % 10000 == 0:
                print(f"Generated {i} rows...")

    # Get file size
    file_size = sys.getsizeof(gz_file) if hasattr(gz_file, "getsize") else 0
    print(f"\n✓ Generated {NUM_ROWS} rows")
    print(f"✓ Output file: {OUTPUT_FILE}")
    print(f"✓ File size: {file_size:,} bytes")


def main():
    """Main function"""
    start_time = datetime.now()
    print("=" * 60)
    print("CSV Test Data Generator")
    print("=" * 60)
    print(f"Rows: {NUM_ROWS:,}")
    print(f"Output: {OUTPUT_FILE}")
    print(f"Start time: {start_time.strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 60 + "\n")

    generate_csv()

    end_time = datetime.now()
    duration = end_time - start_time
    print(f"\nCompleted in {duration.total_seconds():.2f} seconds")
    print("=" * 60)


if __name__ == "__main__":
    main()
