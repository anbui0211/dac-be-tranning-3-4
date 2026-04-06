# GORM Guide for Beginners

Table of Contents:
1. [GORM Fundamentals](#1-gorm-fundamentals)
2. [Basic CRUD Operations](#2-basic-crud-operations)
3. [Relationships](#3-relationships)
4. [Transactions](#4-transactions)
5. [Hooks & Callbacks](#5-hooks--callbacks)
6. [Advanced Features](#6-advanced-features)
7. [Common Pitfalls & Best Practices](#7-common-pitfalls--best-practices)

---

## 1. GORM Fundamentals

### 1.1 Struct Definition & Tags

GORM uses struct tags to define how your Go structs map to database tables:

```go
type User struct {
    ID        uint      `gorm:"primaryKey"`                    // Primary key
    Name      string    `gorm:"column:name;type:varchar(100);not null"`
    Email     string    `gorm:"uniqueIndex"`                    // Unique index
    Age       int       `gorm:"default:18"`                      // Default value
    IsActive  bool      `gorm:"index"`                           // Regular index
    CreatedAt time.Time `gorm:"autoCreateTime"`                 // Auto set on create
    UpdatedAt time.Time `gorm:"autoUpdateTime"`                 // Auto update on update
    DeletedAt gorm.DeletedAt `gorm:"index"`                      // Soft delete
}
```

**Common Tags:**
- `primaryKey` - Set as primary key
- `autoIncrement` - Auto increment
- `column:name` - Specify column name
- `type:varchar(100)` - Specify data type
- `not null` - Set as NOT NULL
- `unique` - Unique constraint
- `uniqueIndex` - Create unique index
- `index` - Create index
- `default:value` - Default value
- `autoCreateTime` - Auto set on create
- `autoUpdateTime` - Auto update on update
- `size:255` - Size for string fields
- `comment:text` - Column comment

### 1.2 Naming Conventions

GORM follows these naming conventions automatically:

| Go Field | Table Column | Example |
|----------|--------------|---------|
| `ID` | `id` | Primary key |
| `Name` | `name` | Snake case |
| `UserID` | `user_id` | Snake case |
| `CreatedAt` | `created_at` | Snake case |

**Override convention:**
```go
type User struct {
    Name string `gorm:"column:custom_name"`  // Use custom_name instead of name
}
```

### 1.3 Model Methods

Define table name and custom methods:

```go
type User struct {
    ID   uint
    Name string
}

func (User) TableName() string {
    return "app_users"  // Custom table name
}
```

---

## 2. Basic CRUD Operations

### 2.1 Create Operations

**Single Record:**
```go
user := User{Name: "John", Email: "john@example.com"}
result := db.Create(&user)

result.Error        // Error if any
result.RowsAffected // Number of records created
user.ID             // Auto-generated ID
```

**Batch Create:**
```go
users := []User{
    {Name: "John"},
    {Name: "Jane"},
}
result := db.Create(&users)  // Create multiple records
```

**Create with Map/Slice:**
```go
db.Model(&User{}).Create(map[string]interface{}{
    "Name":  "John",
    "Email": "john@example.com",
})
```

**Create from struct pointer:**
```go
user := &User{Name: "John"}
db.Create(user)  // Always use pointer for Create
```

### 2.2 Read Operations

**First (get first record ordered by primary key):**
```go
var user User
db.First(&user)  // SELECT * FROM users ORDER BY id LIMIT 1
```

**Take (get one record without ordering):**
```go
var user User
db.Take(&user)  // SELECT * FROM users LIMIT 1
```

**Last (get last record ordered by primary key):**
```go
var user User
db.Last(&user)  // SELECT * FROM users ORDER BY id DESC LIMIT 1
```

**Find (get multiple records):**
```go
var users []User
db.Find(&users)  // SELECT * FROM users
```

**Where Conditions:**
```go
// String conditions
db.Where("name = ?", "John").First(&user)

// Struct conditions
db.Where(&User{Name: "John"}).First(&user)

// Map conditions
db.Where(map[string]interface{}{"name": "John"}).First(&user)

// Multiple conditions
db.Where("name = ? AND age > ?", "John", 18).Find(&users)
```

**Operators:**
```go
db.Where("name LIKE ?", "%john%").Find(&users)           // LIKE
db.Where("age IN ?", []int{18, 20}).Find(&users)         // IN
db.Where("age BETWEEN ? AND ?", 18, 30).Find(&users)     // BETWEEN
db.Where("created_at > ?", time.Now()).Find(&users)     // Comparisons
```

**Pluck (get single column):**
```go
var names []string
db.Model(&User{}).Pluck("name", &names)
```

**Select specific fields:**
```go
db.Select("name", "email").Find(&users)
```

**Limit & Offset:**
```go
db.Limit(10).Offset(20).Find(&users)  // Pagination
```

**Order:**
```go
db.Order("age desc").Find(&users)
db.Order("age asc, name desc").Find(&users)
```

### 2.3 Update Operations

**Save (update all fields including zero values):**
```go
db.Save(&user)  // UPDATE users SET name='?', email='?', age=0 WHERE id=?
```

**Updates (update selected fields):**
```go
db.Model(&user).Updates(User{Name: "New Name", Age: 25})

db.Model(&user).Updates(map[string]interface{}{
    "name": "New Name",
    "age":  25,
})
```

**UpdateColumn (update single column without callbacks):**
```go
db.Model(&user).UpdateColumn("name", "New Name")
```

**Update with Where:**
```go
db.Model(&User{}).Where("active = ?", true).Update("name", "Active User")
```

**Batch Updates:**
```go
db.Table("users").Where("age > ?", 18).Update("status", "adult")
```

### 2.4 Delete Operations

**Delete record:**
```go
db.Delete(&user)  // Soft delete (set deleted_at)
```

**Permanent Delete:**
```go
db.Unscoped().Delete(&user)  // Hard delete
```

**Batch Delete:**
```go
db.Where("age < ?", 18).Delete(&User{})
```

**Delete with Where:**
```go
db.Where("name = ?", "John").Delete(&User{})
```

**SQL Delete:**
```go
db.Exec("DELETE FROM users WHERE age < ?", 18)
```

---

## 3. Relationships

### 3.1 BelongsTo (Many-to-One)

**Define:**
```go
type User struct {
    gorm.Model
    Name  string
    Email string
}

type Profile struct {
    gorm.Model
    UserID uint   // Foreign key
    User   User   `gorm:"foreignKey:UserID"`
    Bio    string
}
```

**Usage:**
```go
var profile Profile
db.Preload("User").First(&profile, 1)

fmt.Println(profile.User.Name)  // Access related user
```

**Create with BelongsTo:**
```go
user := User{Name: "John"}
profile := Profile{
    User: user,  // GORM will auto-fill UserID
    Bio:  "Developer",
}
db.Create(&profile)
```

### 3.2 HasOne (One-to-One)

**Define:**
```go
type User struct {
    gorm.Model
    Name    string
    Profile Profile `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

type Profile struct {
    gorm.Model
    UserID uint   // Foreign key
    Bio    string
}
```

**Usage:**
```go
var user User
db.Preload("Profile").First(&user, 1)

fmt.Println(user.Profile.Bio)
```

### 3.3 HasMany (One-to-Many)

**Define:**
```go
type User struct {
    gorm.Model
    Name    string
    Posts   []Post `gorm:"foreignKey:UserID"`
}

type Post struct {
    gorm.Model
    Title  string
    UserID uint // Foreign key
}
```

**Usage:**
```go
var user User
db.Preload("Posts").First(&user, 1)

for _, post := range user.Posts {
    fmt.Println(post.Title)
}
```

### 3.4 Many2Many

**Define:**
```go
type User struct {
    gorm.Model
    Name   string
    Roles  []Role `gorm:"many2many:user_roles;"`
}

type Role struct {
    gorm.Model
    Name string
}
```

**Usage:**
```go
var user User
db.Preload("Roles").First(&user, 1)

for _, role := range user.Roles {
    fmt.Println(role.Name)
}
```

**Append association:**
```go
db.Model(&user).Association("Roles").Append(&role1, &role2)
```

**Replace association:**
```go
db.Model(&user).Association("Roles").Replace(&role1, &role2)
```

**Delete association:**
```go
db.Model(&user).Association("Roles").Delete(&role1)
```

**Clear association:**
```go
db.Model(&user).Association("Roles").Clear()
```

### 3.5 Eager Loading (Preload)

**Preload single relationship:**
```go
var user User
db.Preload("Profile").First(&user)
```

**Preload multiple relationships:**
```go
db.Preload("Profile").Preload("Posts").First(&user)
```

**Preload with conditions:**
```go
db.Preload("Posts", "status = ?", "published").First(&user)
```

**Nested Preload:**
```go
db.Preload("Posts.Comments").First(&user)
```

**Preload vs Lazy Loading:**
```go
// Eager loading (Preload) - Load in single query
var user User
db.Preload("Profile").First(&user)

// Lazy loading - Load when accessed (N+1 problem!)
var user User
db.First(&user)
var profile Profile
db.Where("user_id = ?", user.ID).First(&profile)  // Additional query
```

### 3.6 Joins

**Inner Join:**
```go
db.Joins("JOIN profiles ON profiles.user_id = users.id").Find(&users)
```

**Left Join:**
```go
db.Joins("LEFT JOIN profiles ON profiles.user_id = users.id").Find(&users)
```

**Join with Preload (eager loading with join):**
```go
db.Joins("Profile").Find(&users)  // Load Profile with join
```

---

## 4. Transactions

### 4.1 Basic Transaction

```go
tx := db.Begin()
defer func() {
    if r := recover(); r != nil {
        tx.Rollback()
    }
}()

if err := tx.Error; err != nil {
    return err
}

// Perform operations
if err := tx.Create(&user).Error; err != nil {
    tx.Rollback()
    return err
}

if err := tx.Create(&profile).Error; err != nil {
    tx.Rollback()
    return err
}

// Commit transaction
if err := tx.Commit().Error; err != nil {
    return err
}

return nil
```

### 4.2 Transaction in Repository

```go
func (r *Repository) CreateUserWithProfile(ctx context.Context, user *User, profile *Profile) error {
    tx := r.db.WithContext(ctx).Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
        }
    }()

    if err := tx.Error; err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }

    if err := tx.Create(user).Error; err != nil {
        tx.Rollback()
        return fmt.Errorf("failed to create user: %w", err)
    }

    profile.UserID = user.ID
    if err := tx.Create(profile).Error; err != nil {
        tx.Rollback()
        return fmt.Errorf("failed to create profile: %w", err)
    }

    if err := tx.Commit().Error; err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }

    return nil
}
```

### 4.3 Nested Transactions

```go
tx := db.Begin()

// Create savepoint
tx.SavePoint("sp1")

// Perform operations...

// Rollback to savepoint
tx.RollbackTo("sp1")

// Continue with transaction
tx.Commit()
```

### 4.4 Transaction Context

```go
// Transaction with context
tx := db.WithContext(ctx).Begin()

// Use context in all operations
tx.Create(&user)  // Uses ctx for timeout/cancellation

tx.Commit()
```

---

## 5. Hooks & Callbacks

### 5.1 Object Lifecycle

GORM hooks run at different stages of object lifecycle:

```
BeforeCreate  → Create  → AfterCreate
BeforeUpdate  → Update  → AfterUpdate
BeforeDelete  → Delete  → AfterDelete
BeforeFind   → Find   → AfterFind
```

### 5.2 Define Hooks

```go
type User struct {
    gorm.Model
    Name  string
    Email string
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
    fmt.Println("Before creating user...")
    if u.Name == "" {
        return errors.New("name cannot be empty")
    }
    return nil
}

func (u *User) AfterCreate(tx *gorm.DB) error {
    fmt.Println("After creating user...")
    return nil
}

func (u *User) BeforeUpdate(tx *gorm.DB) error {
    fmt.Println("Before updating user...")
    return nil
}

func (u *User) AfterUpdate(tx *gorm.DB) error {
    fmt.Println("After updating user...")
    return nil
}

func (u *User) BeforeDelete(tx *gorm.DB) error {
    fmt.Println("Before deleting user...")
    return nil
}

func (u *User) AfterDelete(tx *gorm.DB) error {
    fmt.Println("After deleting user...")
    return nil
}
```

### 5.3 Skip Hooks

```go
db.Session(&gorm.Session{SkipHooks: true}).Create(&user)
db.Session(&gorm.Session{SkipHooks: true}).Update(&user)
```

---

## 6. Advanced Features

### 6.1 Scopes

Define reusable query logic:

```go
// Define scope
func ActiveUsers(db *gorm.DB) *gorm.DB {
    return db.Where("is_active = ?", true)
}

func UsersOlderThan(age int) func(db *gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        return db.Where("age > ?", age)
    }
}

// Use scope
db.Scopes(ActiveUsers).Find(&users)
db.Scopes(ActiveUsers, UsersOlderThan(18)).Find(&users)
```

### 6.2 Conventions & Naming

**Table Naming:**
- By default: StructName → plural_snake_case (User → users)
- Custom: `TableName()` method

**Column Naming:**
- By default: CamelCase → snake_case (UserName → user_name)
- Custom: `gorm:"column:custom_name"` tag

**Foreign Key Naming:**
- By default: StructName + ID (User → user_id)
- Custom: `gorm:"foreignKey:CustomID"` tag

### 6.3 Concurrency (Locking)

**Pessimistic Locking:**
```go
// Lock for update
tx := db.Begin()
user := User{}
tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, 1)
tx.Commit()
```

**Optimistic Locking:**
```go
type Product struct {
    gorm.Model
    Name      string
    Quantity  int
    Version   int `gorm:"version"`  // Optimistic lock
}

db.Save(&product)  // Will check version on update
```

### 6.4 Raw SQL & Custom Queries

**Raw SQL:**
```go
db.Raw("SELECT * FROM users WHERE age > ?", 18).Scan(&users)
```

**Exec SQL:**
```go
db.Exec("UPDATE users SET age = age + 1 WHERE id = ?", 1)
```

**Named Parameters:**
```go
db.Raw("SELECT * FROM users WHERE name = @name", sql.Named("name", "John")).Scan(&users)
```

### 6.5 Performance Optimization

**Use Indexes:**
```go
type User struct {
    Email string `gorm:"uniqueIndex"`  // Create index
    Age   int    `gorm:"index"`         // Create index
}
```

**Batch Operations:**
```go
// Batch create (better performance than loop)
users := make([]User, 100)
for i := range users {
    users[i].Name = fmt.Sprintf("User%d", i)
}
db.CreateInBatches(users, 50)  // Batch size 50
```

**Select Only Needed Fields:**
```go
db.Select("name", "email").Find(&users)  // Better than SELECT *
```

**Use Preload instead of separate queries:**
```go
// Good - Single query with preload
db.Preload("Profile").Find(&users)

// Bad - N+1 queries
db.Find(&users)
for _, user := range users {
    db.Where("user_id = ?", user.ID).First(&profile)
}
```

**Use Connection Pooling:**
```go
sqlDB, _ := db.DB()
sqlDB.SetMaxIdleConns(10)
sqlDB.SetMaxOpenConns(100)
sqlDB.SetConnMaxLifetime(time.Hour)
```

---

## 7. Common Pitfalls & Best Practices

### 7.1 N+1 Query Problem

**Problem:**
```go
// Bad - N+1 queries
var users []User
db.Find(&users)                    // 1 query
for _, user := range users {
    var profile Profile
    db.Where("user_id = ?", user.ID).First(&profile)  // N queries
}
```

**Solution:**
```go
// Good - Eager loading
var users []User
db.Preload("Profile").Find(&users)  // 1 query with JOIN
```

### 7.2 Pointer vs Value Types

**When to use pointers:**
- Optional fields (can be null)
- Update operations (zero value handling)
- Foreign keys (can be null)

**Example:**
```go
type User struct {
    Name     string  `gorm:"not null"`      // Required
    Age      int     `gorm:"not null"`      // Required
    Bio      *string `gorm:"type:text"`     // Optional (can be null)
    Profile  *Profile                      // Optional relationship
}

// Update with pointers (only update non-nil fields)
db.Model(&user).Updates(User{Age: 25})  // Won't update Age if 25
```

### 7.3 Null Handling

**GORM and NULL values:**
```go
// String fields default to NULL
type User struct {
    Name *string `gorm:"type:varchar(100)"`  // Can be NULL
    Bio  string  `gorm:"type:text"`         // Not NULL (default "")
}

// Check for NULL
if user.Name == nil {
    fmt.Println("Name is NULL")
}
```

**Use pointers for nullable fields:**
```go
type User struct {
    Age      int      `gorm:"not null"`       // Not NULL
    Bio      *string  `gorm:"type:text"`      // Can be NULL
    IsActive *bool    `gorm:"default:true"`   // Can be NULL
}
```

### 7.4 Context Management

**Always use context:**
```go
// Good - With context
db.WithContext(ctx).First(&user, 1)

// Bad - No context (can't timeout/cancel)
db.First(&user, 1)
```

**Context in transactions:**
```go
tx := db.WithContext(ctx).Begin()
defer func() {
    if r := recover(); r != nil {
        tx.Rollback()
    }
}()

// Use tx with context
tx.Create(&user)  // Inherits context from tx
```

### 7.5 Connection Pooling

**Configure connection pool:**
```go
sqlDB, err := db.DB()
if err != nil {
    log.Fatal(err)
}

sqlDB.SetMaxIdleConns(10)          // Idle connections in pool
sqlDB.SetMaxOpenConns(100)         // Max open connections
sqlDB.SetConnMaxLifetime(time.Hour)  // Connection lifetime
```

### 7.6 Testing Patterns

**Mock GORM DB:**
```go
import (
    "testing"
    "github.com/DATA-DOG/go-sqlmock"
    "github.com/stretchr/testify/assert"
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
)

func TestCreateUser(t *testing.T) {
    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()

    gormDB, err := gorm.Open(mysql.New(mysql.Config{
        Conn: db,
    }), &gorm.Config{})
    if err != nil {
        t.Fatal(err)
    }

    mock.ExpectBegin()
    mock.ExpectExec("INSERT INTO users").
        WithArgs("John", "john@example.com").
        WillReturnResult(sqlmock.NewResult(1, 1))
    mock.ExpectCommit()

    user := User{Name: "John", Email: "john@example.com"}
    err = gormDB.Create(&user).Error

    assert.NoError(t, err)
    assert.Equal(t, uint(1), user.ID)
}
```

### 7.7 Migration Best Practices

**Version control migrations:**
```sql
-- 000001_create_users_table.up.sql
CREATE TABLE users (
    id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL
);

-- 000001_create_users_table.down.sql
DROP TABLE IF EXISTS users;
```

**Auto migration (for development):**
```go
db.AutoMigrate(&User{}, &Profile{}, &Post{})
```

### 7.8 Error Handling

**Always check errors:**
```go
result := db.Create(&user)
if result.Error != nil {
    log.Printf("Failed to create user: %v", result.Error)
    return result.Error
}

if result.RowsAffected == 0 {
    log.Println("No rows affected")
}
```

**Handle specific errors:**
```go
if errors.Is(err, gorm.ErrRecordNotFound) {
    return "User not found"
}

if errors.Is(err, gorm.ErrDuplicatedKey) {
    return "Duplicate entry"
}
```

---

## Summary

### Key Takeaways:

1. **Use Struct Tags** - Define column names, types, constraints clearly
2. **Eager Loading** - Use Preload to avoid N+1 queries
3. **Transactions** - Always use transactions for multi-step operations
4. **Context** - Pass context for timeout and cancellation
5. **Hooks** - Use hooks for validation and side effects
6. **Pointers** - Use pointers for optional/nullable fields
7. **Scopes** - Create reusable query logic
8. **Indexing** - Use indexes for frequently queried columns
9. **Connection Pool** - Configure for optimal performance
10. **Error Handling** - Always check and handle errors properly

### Further Reading:

- [GORM Official Documentation](https://gorm.io/docs/)
- [GORM Examples](https://github.com/go-gorm/examples)
- [Go Database SQL](https://pkg.go.dev/database/sql)

---

## Quick Reference

### Common Operations:

| Operation | Code |
|-----------|------|
| Create | `db.Create(&user)` |
| First | `db.First(&user)` |
| Find | `db.Find(&users)` |
| Update | `db.Model(&user).Updates(User{Name: "New"})` |
| Delete | `db.Delete(&user)` |
| Preload | `db.Preload("Profile").First(&user)` |
| Transaction | `tx := db.Begin(); tx.Commit()` |
| Where | `db.Where("name = ?", "John").First(&user)` |
| With Context | `db.WithContext(ctx).First(&user)` |
| Raw SQL | `db.Raw("SELECT * FROM users").Scan(&users)` |

### Common Tags:

| Tag | Purpose |
|-----|---------|
| `primaryKey` | Primary key |
| `autoIncrement` | Auto increment |
| `column:name` | Custom column name |
| `type:varchar(100)` | Specify data type |
| `not null` | NOT NULL constraint |
| `unique` | Unique constraint |
| `uniqueIndex` | Unique index |
| `index` | Regular index |
| `default:value` | Default value |
| `autoCreateTime` | Auto set on create |
| `autoUpdateTime` | Auto update on update |
| `foreignKey:ID` | Specify foreign key |
| `many2many:table` | Many-to-many relationship |

---

**End of GORM Guide**
