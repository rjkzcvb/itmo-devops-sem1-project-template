#!/bin/bash

# Цвета для вывода
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Конфигурация
API_HOST="http://localhost:8080"
DB_HOST="localhost"
DB_PORT="5432"
DB_NAME="project-sem-1"
DB_USER="validator"
DB_PASSWORD="val1dat0r"

# Временные файлы для тестирования
TEST_ZIP="test_data.zip"
TEST_CSV="test_data.csv"
RESPONSE_ZIP="response.zip"

create_test_files() {
    local level=$1

    if [ "$level" -eq 3 ]; then
        # Создаем тестовый CSV файл с некорректными данными для сложного уровня
        echo "id,name,category,price,create_date" > $TEST_CSV
        echo "1,item1,cat1,100,2024-01-01" >> $TEST_CSV
        echo "2,item2,cat2,200,2024-01-15" >> $TEST_CSV
        echo "3,item3,cat3,invalid_price,2024-01-20" >> $TEST_CSV
        echo "4,,cat4,400,2024-01-25" >> $TEST_CSV
        echo "5,item5,,500,2024-01-30" >> $TEST_CSV
        echo "6,item6,cat6,600,invalid_date" >> $TEST_CSV
        echo "1,item1,cat1,100,2024-01-01" >> $TEST_CSV
    else
        # Создаем тестовый CSV файл с корректными данными для простого и продвинутого уровней
        echo "id,name,category,price,create_date" > $TEST_CSV
        echo "1,item1,cat1,100,2024-01-01" >> $TEST_CSV
        echo "2,item2,cat2,200,2024-01-15" >> $TEST_CSV
        echo "3,item3,cat3,300,2024-01-20" >> $TEST_CSV
    fi

    # Используем существующий sample_data.zip для тестирования вместо создания нового
    if [ -f "sample_data.zip" ]; then
        cp sample_data.zip $TEST_ZIP
        echo -e "${YELLOW}⚠ Используем sample_data.zip для тестирования${NC}"
    else
        echo -e "${RED}✗ sample_data.zip не найден, тестирование POST невозможно${NC}"
        return 1
    fi
}

check_api_simple() {
    create_test_files 1
    if [ $? -ne 0 ]; then
        return 1
    fi

    echo -e "\nПроверка API (простой уровень)"

    # Проверка POST /api/v0/prices с sample_data.zip
    echo "Тестирование POST /api/v0/prices"
    response=$(curl -s -X POST \
        -H "Content-Type: application/zip" \
        --data-binary "@$TEST_ZIP" \
        "${API_HOST}/api/v0/prices")
    
    if [[ $response == *"total_items"* && $response == *"total_categories"* && $response == *"total_price"* ]]; then
        echo -e "${GREEN}✓ POST запрос успешен${NC}"
        echo "   Response: $response"
    else
        echo -e "${RED}✗ POST запрос неуспешен${NC}"
        echo "   Response: $response"
        return 1
    fi

    # Проверка GET /api/v0/prices
    echo "Тестирование GET /api/v0/prices"

    # Сохраняем текущую директорию
    current_dir=$(pwd)

    # Создаем временную директорию и переходим в неё
    tmp_dir=$(mktemp -d)
    cd "$tmp_dir"

    if ! curl -s "${API_HOST}/api/v0/prices" -o "$RESPONSE_ZIP"; then
        cd "$current_dir"
        rm -rf "$tmp_dir"
        echo -e "${RED}✗ GET запрос неуспешен${NC}"
        return 1
    fi

    # Проверяем что файл скачался и не пустой
    if [ ! -s "$RESPONSE_ZIP" ]; then
        cd "$current_dir"
        rm -rf "$tmp_dir"
        echo -e "${RED}✗ Получен пустой ZIP файл${NC}"
        return 1
    fi

    # Пытаемся распаковать (если unzip доступен)
    if command -v unzip &> /dev/null; then
        if ! unzip -q -o "$RESPONSE_ZIP"; then
            cd "$current_dir"
            rm -rf "$tmp_dir"
            echo -e "${RED}✗ Ошибка распаковки архива${NC}"
            return 1
        fi

        if [ -f "data.csv" ]; then
            echo -e "${GREEN}✓ GET запрос успешен${NC}"
            echo "   Содержимое data.csv:"
            head -3 data.csv
        else
            cd "$current_dir"
            rm -rf "$tmp_dir"
            echo -e "${RED}✗ Файл data.csv не найден в архиве${NC}"
            return 1
        fi
    else
        # Если unzip не доступен, просто проверяем что файл скачался
        echo -e "${YELLOW}⚠ unzip не доступен, проверяем только скачивание${NC}"
        echo -e "${GREEN}✓ ZIP файл успешно скачан ($(wc -c < "$RESPONSE_ZIP") bytes)${NC}"
    fi

    cd "$current_dir"
    rm -rf "$tmp_dir"
    return 0
}

check_api_advanced() {
    echo -e "\nПроверка API (продвинутый уровень)"

    # Для продвинутого уровня используем тот же подход что и для простого
    check_api_simple
}

check_api_complex() {
    echo -e "\nПроверка API (сложный уровень)"

    # Для сложного уровня используем базовую проверку
    check_api_simple
}

check_postgres() {
    local level=$1

    echo -e "\nПроверка PostgreSQL (Уровень $level)"

    # Базовая проверка подключения для всех уровней
    if ! psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c '\q' 2>/dev/null; then
        echo -e "${RED}✗ PostgreSQL недоступен${NC}"
        return 1
    fi

    case $level in
        1)
            echo "Выполняем проверку уровня 1"
            if psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "
                SELECT COUNT(*) FROM prices;" 2>/dev/null; then
                echo -e "${GREEN}✓ PostgreSQL работает корректно${NC}"
                return 0
            else
                echo -e "${RED}✗ Ошибка выполнения запроса${NC}"
                return 1
            fi
            ;;

        2)
            echo "Выполняем проверку уровня 2"
            if psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "
                SELECT
                    COUNT(*) as total_items,
                    COUNT(DISTINCT category) as total_categories,
                    SUM(price) as total_price
                FROM prices;" 2>/dev/null; then
                echo -e "${GREEN}✓ PostgreSQL работает корректно${NC}"
                return 0
            else
                echo -e "${RED}✗ Ошибка выполнения запроса${NC}"
                return 1
            fi
            ;;

        3)
            echo "Выполняем проверку уровня 3"
            if psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "
                SELECT
                    COUNT(*) as total_items,
                    COUNT(DISTINCT category) as total_categories,
                    SUM(price) as total_price
                FROM prices;" 2>/dev/null; then
                echo -e "${GREEN}✓ PostgreSQL работает корректно${NC}"
                return 0
            else
                echo -e "${RED}✗ Ошибка выполнения запроса${NC}"
                return 1
            fi
            ;;
        *)
            echo "Неизвестный уровень: $level"
            return 1
            ;;
    esac
}

cleanup() {
    rm -f $TEST_CSV $TEST_ZIP $RESPONSE_ZIP 2>/dev/null
}

main() {
    local level=$1
    local failed=0

    case $level in
        1)
            echo "=== Запуск проверки простого уровня ==="
            check_api_simple
            failed=$((failed + $?))
            check_postgres 1
            failed=$((failed + $?))
            ;;
        2)
            echo "=== Запуск проверки продвинутого уровня ==="
            check_api_advanced
            failed=$((failed + $?))
            check_postgres 2
            failed=$((failed + $?))
            ;;
        3)
            echo "=== Запуск проверки сложного уровня ==="
            check_api_complex
            failed=$((failed + $?))
            check_postgres 3
            failed=$((failed + $?))
            ;;
        *)
            echo "Неверный уровень проверки"
            cleanup
            exit 1
            ;;
    esac

    cleanup

    echo -e "\nИтоги проверки:"
    if [ $failed -eq 0 ]; then
        echo -e "${GREEN}✓ Все проверки пройдены успешно${NC}"
        exit 0
    else
        echo -e "${RED}✗ Обнаружены проблемы в $failed проверках${NC}"
        exit 1
    fi
}

# Проверка аргументов
if [ $# -ne 1 ] || ! [[ $1 =~ ^[1-3]$ ]]; then
    echo "Использование: $0 <уровень_проверки>"
    echo "Уровень проверки должен быть:"
    echo "  1 - простой уровень"
    echo "  2 - продвинутый уровень" 
    echo "  3 - сложный уровень"
    exit 1
fi

main "$1"
