package oci8

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestConnect checks basic invalid connection
func TestConnect(t *testing.T) {
	if TestDisableDatabase {
		t.SkipNow()
	}

	OCI8Driver.Logger = log.New(os.Stderr, "oci8 ", log.Ldate|log.Ltime|log.LUTC|log.Llongfile)

	// invalid
	db, err := sql.Open("oci8", TestHostInvalid)
	if err != nil {
		t.Fatal("open error:", err)
	}
	if db == nil {
		t.Fatal("db is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	err = db.PingContext(ctx)
	cancel()
	if err == nil || err != driver.ErrBadConn {
		t.Fatalf("ping error - received: %v - expected: %v", err, driver.ErrBadConn)
	}

	err = db.Close()
	if err != nil {
		t.Fatal("close error:", err)
	}

	// wrong username
	db, err = sql.Open("oci8", "dFQXYoApiU2YbquMQnfPyqxR2kAoeuWngDvtTpl3@"+TestHostValid)
	if err != nil {
		t.Fatal("open error:", err)
	}
	if db == nil {
		t.Fatal("db is nil")
	}

	ctx, cancel = context.WithTimeout(context.Background(), TestContextTimeout)
	err = db.PingContext(ctx)
	cancel()
	if err == nil || err != driver.ErrBadConn {
		t.Fatalf("ping error - received: %v - expected: %v", err, driver.ErrBadConn)
	}

	err = db.Close()
	if err != nil {
		t.Fatal("close error:", err)
	}

	OCI8Driver.Logger = log.New(ioutil.Discard, "", 0)
}

// TestSelectParallel checks parallel select from dual
func TestSelectParallel(t *testing.T) {
	if TestDisableDatabase {
		t.SkipNow()
	}

	ctx, cancel := context.WithTimeout(context.Background(), TestContextTimeout)
	stmt, err := TestDB.PrepareContext(ctx, "select :1 from dual")
	cancel()
	if err != nil {
		t.Fatal("prepare error:", err)
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(100)

	for i := 0; i < 100; i++ {
		go func(num int) {
			defer waitGroup.Done()
			var result [][]interface{}
			result, err = testGetRows(t, stmt, []interface{}{num})
			if err != nil {
				t.Fatal("get rows error:", err)
			}
			if result == nil {
				t.Fatal("result is nil")
			}
			if len(result) < 1 {
				t.Fatal("len result less than 1")
			}
			if len(result[0]) < 1 {
				t.Fatal("len result[0] less than 1")
			}
			data, ok := result[0][0].(int64)
			if !ok {
				t.Fatal("result not int64")
			}
			if data != int64(num) {
				t.Fatal("result not equal to:", num)
			}
		}(i)
	}

	waitGroup.Wait()

	err = stmt.Close()
	if err != nil {
		t.Fatal("stmt close error:", err)
	}
}

// TestContextTimeoutBreak checks that ExecContext timeout works
func TestContextTimeoutBreak(t *testing.T) {
	if TestDisableDatabase {
		t.SkipNow()
	}

	// exec
	ctx, cancel := context.WithTimeout(context.Background(), TestContextTimeout)
	stmt, err := TestDB.PrepareContext(ctx, "begin SYS.DBMS_LOCK.SLEEP(1); end;")
	cancel()
	if err != nil {
		t.Fatal("prepare error:", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 200*time.Millisecond)
	_, err = stmt.ExecContext(ctx)
	cancel()
	expected := "ORA-01013"
	if err == nil || len(err.Error()) < len(expected) || err.Error()[:len(expected)] != expected {
		t.Fatalf("stmt exec - expected: %v - received: %v", expected, err)
	}

	err = stmt.Close()
	if err != nil {
		t.Fatal("stmt close error:", err)
	}

	// query
	ctx, cancel = context.WithTimeout(context.Background(), TestContextTimeout)
	stmt, err = TestDB.PrepareContext(ctx, "select SLEEP_SECONDS(1) from dual")
	cancel()
	if err != nil {
		t.Fatal("prepare error:", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 200*time.Millisecond)
	_, err = stmt.QueryContext(ctx)
	cancel()
	if err == nil || len(err.Error()) < len(expected) || err.Error()[:len(expected)] != expected {
		t.Fatalf("stmt query - expected: %v - received: %v", expected, err)
	}

	err = stmt.Close()
	if err != nil {
		t.Fatal("stmt close error:", err)
	}
}

// TestSelectCast checks cast x from dual works for each SQL types
func TestSelectCast(t *testing.T) {
	if TestDisableDatabase {
		t.SkipNow()
	}

	// https://ss64.com/ora/syntax-datatypes.html

	queryResults := []testQueryResults{

		// VARCHAR2(1)
		testQueryResults{
			query: "select cast (:1 as VARCHAR2(1)) from dual",
			args: [][]interface{}{
				[]interface{}{""},
				[]interface{}{"a"},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{nil}},
				[][]interface{}{[]interface{}{"a"}},
			},
		},

		// VARCHAR2(4000)
		testQueryResults{
			query: "select cast (:1 as VARCHAR2(4000)) from dual",
			args: [][]interface{}{
				[]interface{}{""},
				[]interface{}{"a"},
				[]interface{}{"abc    "},
				[]interface{}{strings.Repeat("a", 10)},
				[]interface{}{strings.Repeat("a", 100)},
				[]interface{}{strings.Repeat("a", 500)},
				[]interface{}{strings.Repeat("a", 1000)},
				[]interface{}{strings.Repeat("a", 1500)},
				[]interface{}{strings.Repeat("a", 2000)},
				[]interface{}{strings.Repeat("a", 3000)},
				[]interface{}{strings.Repeat("a", 4000)},
				[]interface{}{testString1},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{nil}},
				[][]interface{}{[]interface{}{"a"}},
				[][]interface{}{[]interface{}{"abc    "}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 10)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 100)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 500)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 1000)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 1500)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 2000)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 3000)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 4000)}},
				[][]interface{}{[]interface{}{testString1}},
			},
		},

		// NVARCHAR2(1)
		testQueryResults{
			query: "select cast (:1 as NVARCHAR2(1)) from dual",
			args: [][]interface{}{
				[]interface{}{""},
				[]interface{}{"a"},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{nil}},
				[][]interface{}{[]interface{}{"a"}},
			},
		},

		// NVARCHAR2(2000)
		testQueryResults{
			query: "select cast (:1 as NVARCHAR2(2000)) from dual",
			args: [][]interface{}{
				[]interface{}{""},
				[]interface{}{"a"},
				[]interface{}{"abc    "},
				[]interface{}{strings.Repeat("a", 10)},
				[]interface{}{strings.Repeat("a", 100)},
				[]interface{}{strings.Repeat("a", 500)},
				[]interface{}{strings.Repeat("a", 1000)},
				[]interface{}{strings.Repeat("a", 1500)},
				[]interface{}{strings.Repeat("a", 2000)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{nil}},
				[][]interface{}{[]interface{}{"a"}},
				[][]interface{}{[]interface{}{"abc    "}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 10)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 100)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 500)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 1000)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 1500)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 2000)}},
			},
		},

		// CHAR(1)
		testQueryResults{
			query: "select cast (:1 as CHAR(1)) from dual",
			args: [][]interface{}{
				[]interface{}{""},
				[]interface{}{"a"},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{nil}},
				[][]interface{}{[]interface{}{"a"}},
			},
		},

		// CHAR(2000)
		testQueryResults{
			query: "select cast (:1 as CHAR(2000)) from dual",
			args: [][]interface{}{
				[]interface{}{""},
				[]interface{}{"a"},
				[]interface{}{"abc    "},
				[]interface{}{strings.Repeat("a", 10)},
				[]interface{}{strings.Repeat("a", 100)},
				[]interface{}{strings.Repeat("a", 500)},
				[]interface{}{strings.Repeat("a", 1000)},
				[]interface{}{strings.Repeat("a", 1500)},
				[]interface{}{strings.Repeat("a", 2000)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{nil}},
				[][]interface{}{[]interface{}{"a" + strings.Repeat(" ", 1999)}},
				[][]interface{}{[]interface{}{"abc" + strings.Repeat(" ", 1997)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 10) + strings.Repeat(" ", 1990)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 100) + strings.Repeat(" ", 1900)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 500) + strings.Repeat(" ", 1500)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 1000) + strings.Repeat(" ", 1000)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 1500) + strings.Repeat(" ", 500)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 2000)}},
			},
		},

		// NCHAR(1)
		testQueryResults{
			query: "select cast (:1 as NCHAR(1)) from dual",
			args: [][]interface{}{
				[]interface{}{""},
				[]interface{}{"a"},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{nil}},
				[][]interface{}{[]interface{}{"a"}},
			},
		},

		// NCHAR(1000)
		testQueryResults{
			query: "select cast (:1 as NCHAR(1000)) from dual",
			args: [][]interface{}{
				[]interface{}{""},
				[]interface{}{"a"},
				[]interface{}{"abc    "},
				[]interface{}{strings.Repeat("a", 10)},
				[]interface{}{strings.Repeat("a", 100)},
				[]interface{}{strings.Repeat("a", 500)},
				[]interface{}{strings.Repeat("a", 1000)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{nil}},
				[][]interface{}{[]interface{}{"a" + strings.Repeat(" ", 999)}},
				[][]interface{}{[]interface{}{"abc" + strings.Repeat(" ", 997)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 10) + strings.Repeat(" ", 990)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 100) + strings.Repeat(" ", 900)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 500) + strings.Repeat(" ", 500)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 1000)}},
			},
		},

		// NUMBER(38,10)
		testQueryResults{
			query: "select cast (:1 as NUMBER(38,10)) from dual",
			args: [][]interface{}{
				[]interface{}{float64(-99999999999999999999999999.9999999999)},
				[]interface{}{float64(-2147483648)},
				[]interface{}{float64(-123456792)},
				[]interface{}{float64(-1.9873046875)},
				[]interface{}{float64(-1)},
				[]interface{}{float64(-0.76171875)},
				[]interface{}{float64(0)},
				[]interface{}{float64(0.76171875)},
				[]interface{}{float64(1)},
				[]interface{}{float64(1.9873046875)},
				[]interface{}{float64(123456792)},
				[]interface{}{float64(2147483647)},
				[]interface{}{float64(99999999999999999999999999.9999999999)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{float64(-99999999999999999999999999.9999999999)}},
				[][]interface{}{[]interface{}{float64(-2147483648)}},
				[][]interface{}{[]interface{}{float64(-123456792)}},
				[][]interface{}{[]interface{}{float64(-1.9873046875)}},
				[][]interface{}{[]interface{}{float64(-1)}},
				[][]interface{}{[]interface{}{float64(-0.76171875)}},
				[][]interface{}{[]interface{}{float64(0)}},
				[][]interface{}{[]interface{}{float64(0.76171875)}},
				[][]interface{}{[]interface{}{float64(1)}},
				[][]interface{}{[]interface{}{float64(1.9873046875)}},
				[][]interface{}{[]interface{}{float64(123456792)}},
				[][]interface{}{[]interface{}{float64(2147483647)}},
				[][]interface{}{[]interface{}{float64(99999999999999999999999999.9999999999)}},
			},
		},

		// DEC(38,10)
		testQueryResults{
			query: "select cast (:1 as DEC(38,10)) from dual",
			args: [][]interface{}{
				[]interface{}{float64(-99999999999999999999999999.9999999999)},
				[]interface{}{float64(-2147483648)},
				[]interface{}{float64(-123456792)},
				[]interface{}{float64(-1.9873046875)},
				[]interface{}{float64(-1)},
				[]interface{}{float64(-0.76171875)},
				[]interface{}{float64(0)},
				[]interface{}{float64(0.76171875)},
				[]interface{}{float64(1)},
				[]interface{}{float64(1.9873046875)},
				[]interface{}{float64(123456792)},
				[]interface{}{float64(2147483647)},
				[]interface{}{float64(99999999999999999999999999.9999999999)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{float64(-99999999999999999999999999.9999999999)}},
				[][]interface{}{[]interface{}{float64(-2147483648)}},
				[][]interface{}{[]interface{}{float64(-123456792)}},
				[][]interface{}{[]interface{}{float64(-1.9873046875)}},
				[][]interface{}{[]interface{}{float64(-1)}},
				[][]interface{}{[]interface{}{float64(-0.76171875)}},
				[][]interface{}{[]interface{}{float64(0)}},
				[][]interface{}{[]interface{}{float64(0.76171875)}},
				[][]interface{}{[]interface{}{float64(1)}},
				[][]interface{}{[]interface{}{float64(1.9873046875)}},
				[][]interface{}{[]interface{}{float64(123456792)}},
				[][]interface{}{[]interface{}{float64(2147483647)}},
				[][]interface{}{[]interface{}{float64(99999999999999999999999999.9999999999)}},
			},
		},

		// DECIMAL(38,10)
		testQueryResults{
			query: "select cast (:1 as DECIMAL(38,10)) from dual",
			args: [][]interface{}{
				[]interface{}{float64(-99999999999999999999999999.9999999999)},
				[]interface{}{float64(-2147483648)},
				[]interface{}{float64(-123456792)},
				[]interface{}{float64(-1.9873046875)},
				[]interface{}{float64(-1)},
				[]interface{}{float64(-0.76171875)},
				[]interface{}{float64(0)},
				[]interface{}{float64(0.76171875)},
				[]interface{}{float64(1)},
				[]interface{}{float64(1.9873046875)},
				[]interface{}{float64(123456792)},
				[]interface{}{float64(2147483647)},
				[]interface{}{float64(99999999999999999999999999.9999999999)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{float64(-99999999999999999999999999.9999999999)}},
				[][]interface{}{[]interface{}{float64(-2147483648)}},
				[][]interface{}{[]interface{}{float64(-123456792)}},
				[][]interface{}{[]interface{}{float64(-1.9873046875)}},
				[][]interface{}{[]interface{}{float64(-1)}},
				[][]interface{}{[]interface{}{float64(-0.76171875)}},
				[][]interface{}{[]interface{}{float64(0)}},
				[][]interface{}{[]interface{}{float64(0.76171875)}},
				[][]interface{}{[]interface{}{float64(1)}},
				[][]interface{}{[]interface{}{float64(1.9873046875)}},
				[][]interface{}{[]interface{}{float64(123456792)}},
				[][]interface{}{[]interface{}{float64(2147483647)}},
				[][]interface{}{[]interface{}{float64(99999999999999999999999999.9999999999)}},
			},
		},

		// NUMERIC(38,10)
		testQueryResults{
			query: "select cast (:1 as NUMERIC(38,10)) from dual",
			args: [][]interface{}{
				[]interface{}{float64(-99999999999999999999999999.9999999999)},
				[]interface{}{float64(-2147483648)},
				[]interface{}{float64(-123456792)},
				[]interface{}{float64(-1.9873046875)},
				[]interface{}{float64(-1)},
				[]interface{}{float64(-0.76171875)},
				[]interface{}{float64(0)},
				[]interface{}{float64(0.76171875)},
				[]interface{}{float64(1)},
				[]interface{}{float64(1.9873046875)},
				[]interface{}{float64(123456792)},
				[]interface{}{float64(2147483647)},
				[]interface{}{float64(99999999999999999999999999.9999999999)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{float64(-99999999999999999999999999.9999999999)}},
				[][]interface{}{[]interface{}{float64(-2147483648)}},
				[][]interface{}{[]interface{}{float64(-123456792)}},
				[][]interface{}{[]interface{}{float64(-1.9873046875)}},
				[][]interface{}{[]interface{}{float64(-1)}},
				[][]interface{}{[]interface{}{float64(-0.76171875)}},
				[][]interface{}{[]interface{}{float64(0)}},
				[][]interface{}{[]interface{}{float64(0.76171875)}},
				[][]interface{}{[]interface{}{float64(1)}},
				[][]interface{}{[]interface{}{float64(1.9873046875)}},
				[][]interface{}{[]interface{}{float64(123456792)}},
				[][]interface{}{[]interface{}{float64(2147483647)}},
				[][]interface{}{[]interface{}{float64(99999999999999999999999999.9999999999)}},
			},
		},

		// FLOAT
		testQueryResults{
			query: "select cast (:1 as FLOAT) from dual",
			args: [][]interface{}{
				[]interface{}{float64(-288230381928101358902502915674136903680)},
				[]interface{}{float64(-2147483648)},
				[]interface{}{float64(-123456792)},
				[]interface{}{float64(-1.99999988079071044921875)},
				[]interface{}{float64(-1)},
				[]interface{}{float64(-0.00415134616196155548095703125)},
				[]interface{}{float64(0)},
				[]interface{}{float64(0.00415134616196155548095703125)},
				[]interface{}{float64(1)},
				[]interface{}{float64(1.99999988079071044921875)},
				[]interface{}{float64(123456792)},
				[]interface{}{float64(2147483647)},
				[]interface{}{float64(288230381928101358902502915674136903680)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{float64(-288230381928101358902502915674136903680)}},
				[][]interface{}{[]interface{}{float64(-2147483648)}},
				[][]interface{}{[]interface{}{float64(-123456792)}},
				[][]interface{}{[]interface{}{float64(-1.99999988079071044921875)}},
				[][]interface{}{[]interface{}{float64(-1)}},
				[][]interface{}{[]interface{}{float64(-0.00415134616196155548095703125)}},
				[][]interface{}{[]interface{}{float64(0)}},
				[][]interface{}{[]interface{}{float64(0.00415134616196155548095703125)}},
				[][]interface{}{[]interface{}{float64(1)}},
				[][]interface{}{[]interface{}{float64(1.99999988079071044921875)}},
				[][]interface{}{[]interface{}{float64(123456792)}},
				[][]interface{}{[]interface{}{float64(2147483647)}},
				[][]interface{}{[]interface{}{float64(288230381928101358902502915674136903680)}},
			},
		},

		// INTEGER
		testQueryResults{
			query: "select cast (:1 as INTEGER) from dual",
			args: [][]interface{}{
				[]interface{}{int64(-2147483648)},
				[]interface{}{int64(-1)},
				[]interface{}{int64(0)},
				[]interface{}{int64(1)},
				[]interface{}{int64(2147483647)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(-2147483648)}},
				[][]interface{}{[]interface{}{int64(-1)}},
				[][]interface{}{[]interface{}{int64(0)}},
				[][]interface{}{[]interface{}{int64(1)}},
				[][]interface{}{[]interface{}{int64(2147483647)}},
			},
		},

		// INT
		testQueryResults{
			query: "select cast (:1 as INT) from dual",
			args: [][]interface{}{
				[]interface{}{int64(-2147483648)},
				[]interface{}{int64(-1)},
				[]interface{}{int64(0)},
				[]interface{}{int64(1)},
				[]interface{}{int64(2147483647)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(-2147483648)}},
				[][]interface{}{[]interface{}{int64(-1)}},
				[][]interface{}{[]interface{}{int64(0)}},
				[][]interface{}{[]interface{}{int64(1)}},
				[][]interface{}{[]interface{}{int64(2147483647)}},
			},
		},

		// SMALLINT
		testQueryResults{
			query: "select cast (:1 as SMALLINT) from dual",
			args: [][]interface{}{
				[]interface{}{int64(-2147483648)},
				[]interface{}{int64(-1)},
				[]interface{}{int64(0)},
				[]interface{}{int64(1)},
				[]interface{}{int64(2147483647)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(-2147483648)}},
				[][]interface{}{[]interface{}{int64(-1)}},
				[][]interface{}{[]interface{}{int64(0)}},
				[][]interface{}{[]interface{}{int64(1)}},
				[][]interface{}{[]interface{}{int64(2147483647)}},
			},
		},

		// REAL
		testQueryResults{
			query: "select cast (:1 as REAL) from dual",
			args: [][]interface{}{
				[]interface{}{float64(-288230381928101358902502915674136903680)},
				[]interface{}{float64(-2147483648)},
				[]interface{}{float64(-123456792)},
				[]interface{}{float64(-1.99999988079071044921875)},
				[]interface{}{float64(-1)},
				[]interface{}{float64(-0.00415134616196155548095703125)},
				[]interface{}{float64(0)},
				[]interface{}{float64(0.00415134616196155548095703125)},
				[]interface{}{float64(1)},
				[]interface{}{float64(1.99999988079071044921875)},
				[]interface{}{float64(123456792)},
				[]interface{}{float64(2147483647)},
				[]interface{}{float64(288230381928101358902502915674136903680)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{float64(-288230381928101358902502915674136903680)}},
				[][]interface{}{[]interface{}{float64(-2147483648)}},
				[][]interface{}{[]interface{}{float64(-123456792)}},
				[][]interface{}{[]interface{}{float64(-1.99999988079071044921875)}},
				[][]interface{}{[]interface{}{float64(-1)}},
				[][]interface{}{[]interface{}{float64(-0.00415134616196155548095703125)}},
				[][]interface{}{[]interface{}{float64(0)}},
				[][]interface{}{[]interface{}{float64(0.00415134616196155548095703125)}},
				[][]interface{}{[]interface{}{float64(1)}},
				[][]interface{}{[]interface{}{float64(1.99999988079071044921875)}},
				[][]interface{}{[]interface{}{float64(123456792)}},
				[][]interface{}{[]interface{}{float64(2147483647)}},
				[][]interface{}{[]interface{}{float64(288230381928101358902502915674136903680)}},
			},
		},

		// BINARY_FLOAT
		testQueryResults{
			query: "select cast (:1 as BINARY_FLOAT) from dual",
			args: [][]interface{}{
				[]interface{}{float64(-288230381928101358902502915674136903680)},
				[]interface{}{float64(-2147483648)},
				[]interface{}{float64(-123456792)},
				[]interface{}{float64(-1.99999988079071044921875)},
				[]interface{}{float64(-1)},
				[]interface{}{float64(-0.00415134616196155548095703125)},
				[]interface{}{float64(0)},
				[]interface{}{float64(0.00415134616196155548095703125)},
				[]interface{}{float64(1)},
				[]interface{}{float64(1.99999988079071044921875)},
				[]interface{}{float64(123456792)},
				[]interface{}{float64(2147483648)},
				[]interface{}{float64(288230381928101358902502915674136903680)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{float64(-288230381928101358902502915674136903680)}},
				[][]interface{}{[]interface{}{float64(-2147483648)}},
				[][]interface{}{[]interface{}{float64(-123456792)}},
				[][]interface{}{[]interface{}{float64(-1.99999988079071044921875)}},
				[][]interface{}{[]interface{}{float64(-1)}},
				[][]interface{}{[]interface{}{float64(-0.00415134616196155548095703125)}},
				[][]interface{}{[]interface{}{float64(0)}},
				[][]interface{}{[]interface{}{float64(0.00415134616196155548095703125)}},
				[][]interface{}{[]interface{}{float64(1)}},
				[][]interface{}{[]interface{}{float64(1.99999988079071044921875)}},
				[][]interface{}{[]interface{}{float64(123456792)}},
				[][]interface{}{[]interface{}{float64(2147483648)}},
				[][]interface{}{[]interface{}{float64(288230381928101358902502915674136903680)}},
			},
		},

		// BINARY_DOUBLE
		testQueryResults{
			query: "select cast (:1 as BINARY_DOUBLE) from dual",
			args: [][]interface{}{
				[]interface{}{float64(-288230381928101358902502915674136903680)},
				[]interface{}{float64(-2147483648)},
				[]interface{}{float64(-123456792)},
				[]interface{}{float64(-1.99999988079071044921875)},
				[]interface{}{float64(-1)},
				[]interface{}{float64(-0.00415134616196155548095703125)},
				[]interface{}{float64(0)},
				[]interface{}{float64(0.00415134616196155548095703125)},
				[]interface{}{float64(1)},
				[]interface{}{float64(1.99999988079071044921875)},
				[]interface{}{float64(123456792)},
				[]interface{}{float64(2147483647)},
				[]interface{}{float64(288230381928101358902502915674136903680)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{float64(-288230381928101358902502915674136903680)}},
				[][]interface{}{[]interface{}{float64(-2147483648)}},
				[][]interface{}{[]interface{}{float64(-123456792)}},
				[][]interface{}{[]interface{}{float64(-1.99999988079071044921875)}},
				[][]interface{}{[]interface{}{float64(-1)}},
				[][]interface{}{[]interface{}{float64(-0.00415134616196155548095703125)}},
				[][]interface{}{[]interface{}{float64(0)}},
				[][]interface{}{[]interface{}{float64(0.00415134616196155548095703125)}},
				[][]interface{}{[]interface{}{float64(1)}},
				[][]interface{}{[]interface{}{float64(1.99999988079071044921875)}},
				[][]interface{}{[]interface{}{float64(123456792)}},
				[][]interface{}{[]interface{}{float64(2147483647)}},
				[][]interface{}{[]interface{}{float64(288230381928101358902502915674136903680)}},
			},
		},

		// TIMESTAMP(9) WITH TIME ZONE
		testQueryResults{
			query: "select cast (:1 as TIMESTAMP(9) WITH TIME ZONE) from dual",
			args: [][]interface{}{
				[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, time.UTC)},
				[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocUTC)},
				[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocGMT)},
				[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocEST)},
				[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocMST)},
				[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocNZ)},
				[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)},
				[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocUTC)},
				[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocGMT)},
				[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocEST)},
				[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocMST)},
				// TOFIX: ORA-08192: Flashback Table operation is not allowed on fixed tables
				// []interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocNZ)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, time.UTC)}},
				[][]interface{}{[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocUTC)}},
				[][]interface{}{[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocGMT)}},
				[][]interface{}{[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocEST)}},
				[][]interface{}{[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocMST)}},
				[][]interface{}{[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocNZ)}},
				[][]interface{}{[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)}},
				[][]interface{}{[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocUTC)}},
				[][]interface{}{[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocGMT)}},
				[][]interface{}{[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocEST)}},
				[][]interface{}{[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocMST)}},
				// TOFIX: ORA-08192: Flashback Table operation is not allowed on fixed tables
				// [][]interface{}{[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocNZ)}},
			},
		},

		// INTERVAL DAY TO MONTH - YEAR
		testQueryResults{
			query: "select NUMTOYMINTERVAL(:1, 'YEAR') from dual",
			args: [][]interface{}{
				[]interface{}{-2},
				[]interface{}{-1},
				[]interface{}{1},
				[]interface{}{2},
				[]interface{}{float64(1.25)},
				[]interface{}{float64(1.5)},
				[]interface{}{float64(2.75)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(-24)}},
				[][]interface{}{[]interface{}{int64(-12)}},
				[][]interface{}{[]interface{}{int64(12)}},
				[][]interface{}{[]interface{}{int64(24)}},
				[][]interface{}{[]interface{}{int64(15)}},
				[][]interface{}{[]interface{}{int64(18)}},
				[][]interface{}{[]interface{}{int64(33)}},
			},
		},

		// INTERVAL DAY TO MONTH - MONTH
		testQueryResults{
			query: "select NUMTOYMINTERVAL(:1, 'MONTH') from dual",
			args: [][]interface{}{
				[]interface{}{-2},
				[]interface{}{-1},
				[]interface{}{1},
				[]interface{}{2},
				[]interface{}{2.1},
				[]interface{}{2.9},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(-2)}},
				[][]interface{}{[]interface{}{int64(-1)}},
				[][]interface{}{[]interface{}{int64(1)}},
				[][]interface{}{[]interface{}{int64(2)}},
				[][]interface{}{[]interface{}{int64(2)}},
				[][]interface{}{[]interface{}{int64(3)}},
			},
		},

		// INTERVAL DAY TO SECOND - DAY
		testQueryResults{
			query: "select NUMTODSINTERVAL(:1, 'DAY') from dual",
			args: [][]interface{}{
				[]interface{}{-2},
				[]interface{}{-1},
				[]interface{}{1},
				[]interface{}{2},
				[]interface{}{1.25},
				[]interface{}{1.5},
				[]interface{}{2.75},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(-172800000000000)}},
				[][]interface{}{[]interface{}{int64(-86400000000000)}},
				[][]interface{}{[]interface{}{int64(86400000000000)}},
				[][]interface{}{[]interface{}{int64(172800000000000)}},
				[][]interface{}{[]interface{}{int64(108000000000000)}},
				[][]interface{}{[]interface{}{int64(129600000000000)}},
				[][]interface{}{[]interface{}{int64(237600000000000)}},
			},
		},

		// INTERVAL DAY TO SECOND - HOUR
		testQueryResults{
			query: "select NUMTODSINTERVAL(:1, 'HOUR') from dual",
			args: [][]interface{}{
				[]interface{}{-2},
				[]interface{}{-1},
				[]interface{}{1},
				[]interface{}{2},
				[]interface{}{1.25},
				[]interface{}{1.5},
				[]interface{}{2.75},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(-7200000000000)}},
				[][]interface{}{[]interface{}{int64(-3600000000000)}},
				[][]interface{}{[]interface{}{int64(3600000000000)}},
				[][]interface{}{[]interface{}{int64(7200000000000)}},
				[][]interface{}{[]interface{}{int64(4500000000000)}},
				[][]interface{}{[]interface{}{int64(5400000000000)}},
				[][]interface{}{[]interface{}{int64(9900000000000)}},
			},
		},

		// INTERVAL DAY TO SECOND - MINUTE
		testQueryResults{
			query: "select NUMTODSINTERVAL(:1, 'MINUTE') from dual",
			args: [][]interface{}{
				[]interface{}{-2},
				[]interface{}{-1},
				[]interface{}{1},
				[]interface{}{2},
				[]interface{}{1.25},
				[]interface{}{1.5},
				[]interface{}{2.75},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(-120000000000)}},
				[][]interface{}{[]interface{}{int64(-60000000000)}},
				[][]interface{}{[]interface{}{int64(60000000000)}},
				[][]interface{}{[]interface{}{int64(120000000000)}},
				[][]interface{}{[]interface{}{int64(75000000000)}},
				[][]interface{}{[]interface{}{int64(90000000000)}},
				[][]interface{}{[]interface{}{int64(165000000000)}},
			},
		},

		// INTERVAL DAY TO SECOND - SECOND
		testQueryResults{
			query: "select NUMTODSINTERVAL(:1, 'SECOND') from dual",
			args: [][]interface{}{
				[]interface{}{-2},
				[]interface{}{-1},
				[]interface{}{1},
				[]interface{}{2},
				[]interface{}{1.25},
				[]interface{}{1.5},
				[]interface{}{2.75},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(-2000000000)}},
				[][]interface{}{[]interface{}{int64(-1000000000)}},
				[][]interface{}{[]interface{}{int64(1000000000)}},
				[][]interface{}{[]interface{}{int64(2000000000)}},
				[][]interface{}{[]interface{}{int64(1250000000)}},
				[][]interface{}{[]interface{}{int64(1500000000)}},
				[][]interface{}{[]interface{}{int64(2750000000)}},
			},
		},

		// RAW(2000)
		testQueryResults{
			query: "select cast (:1 as RAW(2000)) from dual",
			args: [][]interface{}{
				[]interface{}{[]byte{}},
				[]interface{}{[]byte{10}},
				[]interface{}{[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
				[]interface{}{[]byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}},
				[]interface{}{[]byte{245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255}},
				[]interface{}{testByteSlice1},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{nil}},
				[][]interface{}{[]interface{}{[]byte{10}}},
				[][]interface{}{[]interface{}{[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}},
				[][]interface{}{[]interface{}{[]byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}}},
				[][]interface{}{[]interface{}{[]byte{245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255}}},
				[][]interface{}{[]interface{}{testByteSlice1}},
			},
		},

		// CLOB
		testQueryResults{
			query: "select TO_CLOB(:1) from dual",
			args: [][]interface{}{
				[]interface{}{""},
				[]interface{}{"a"},
				[]interface{}{"abc    "},
				[]interface{}{strings.Repeat("a", 100)},
				[]interface{}{strings.Repeat("a", 500)},
				[]interface{}{strings.Repeat("a", 1000)},
				[]interface{}{strings.Repeat("a", 2000)},
				[]interface{}{strings.Repeat("a", 4000)},
				[]interface{}{testString1},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{nil}},
				[][]interface{}{[]interface{}{"a"}},
				[][]interface{}{[]interface{}{"abc    "}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 100)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 500)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 1000)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 2000)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 4000)}},
				[][]interface{}{[]interface{}{testString1}},
			},
		},

		// NCLOB
		testQueryResults{
			query: "select TO_NCLOB(:1) from dual",
			args: [][]interface{}{
				[]interface{}{""},
				[]interface{}{"a"},
				[]interface{}{"abc    "},
				[]interface{}{strings.Repeat("a", 100)},
				[]interface{}{strings.Repeat("a", 500)},
				[]interface{}{strings.Repeat("a", 1000)},
				[]interface{}{strings.Repeat("a", 2000)},
				[]interface{}{strings.Repeat("a", 4000)},
				[]interface{}{testString1},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{nil}},
				[][]interface{}{[]interface{}{"a"}},
				[][]interface{}{[]interface{}{"abc    "}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 100)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 500)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 1000)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 2000)}},
				[][]interface{}{[]interface{}{strings.Repeat("a", 4000)}},
				[][]interface{}{[]interface{}{testString1}},
			},
		},

		// BLOB
		testQueryResults{
			query: "select TO_BLOB(:1) from dual",
			args: [][]interface{}{
				[]interface{}{[]byte{}},
				[]interface{}{[]byte{10}},
				[]interface{}{[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
				[]interface{}{[]byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}},
				[]interface{}{[]byte{245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255}},
				[]interface{}{testByteSlice1},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{nil}},
				[][]interface{}{[]interface{}{[]byte{10}}},
				[][]interface{}{[]interface{}{[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}},
				[][]interface{}{[]interface{}{[]byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}}},
				[][]interface{}{[]interface{}{[]byte{245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255}}},
				[][]interface{}{[]interface{}{testByteSlice1}},
			},
		},
	}

	testRunQueryResults(t, queryResults)

	// ROWID
	ctx, cancel := context.WithTimeout(context.Background(), TestContextTimeout)
	stmt, err := TestDB.PrepareContext(ctx, "select ROWID from dual")
	cancel()
	if err != nil {
		t.Fatal("prepare error:", err)
	}

	var result [][]interface{}
	result, err = testGetRows(t, stmt, nil)
	if err != nil {
		t.Fatal("get rows error:", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result) < 1 {
		t.Fatal("len result less than 1")
	}
	if len(result[0]) < 1 {
		t.Fatal("len result[0] less than 1")
	}
	data, ok := result[0][0].(string)
	if !ok {
		t.Fatal("result not string")
	}
	if len(data) != 18 {
		t.Fatal("result len not equal to 18:", len(data))
	}
}

// TestSelectGoTypes is select :1 from dual for each Go Type
func TestSelectGoTypes(t *testing.T) {
	if TestDisableDatabase {
		t.SkipNow()
	}

	// https://tour.golang.org/basics/11

	queryResults := []testQueryResults{
		// bool
		testQueryResults{
			query: "select :1 from dual",
			args: [][]interface{}{
				[]interface{}{true},
				[]interface{}{false},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(1)}},
				[][]interface{}{[]interface{}{int64(0)}},
			},
		},

		// string
		testQueryResults{
			query: "select :1 from dual",
			args: [][]interface{}{
				[]interface{}{""},
				[]interface{}{"a"},
				[]interface{}{"123"},
				[]interface{}{"1234.567"},
				[]interface{}{"abc      "},
				[]interface{}{"abcdefghijklmnopqrstuvwxyz"},
				[]interface{}{"a b c d e f g h i j k l m n o p q r s t u v w x y z "},
				[]interface{}{"a\nb\nc"},
				[]interface{}{testString1},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{nil}},
				[][]interface{}{[]interface{}{"a"}},
				[][]interface{}{[]interface{}{"123"}},
				[][]interface{}{[]interface{}{"1234.567"}},
				[][]interface{}{[]interface{}{"abc      "}},
				[][]interface{}{[]interface{}{"abcdefghijklmnopqrstuvwxyz"}},
				[][]interface{}{[]interface{}{"a b c d e f g h i j k l m n o p q r s t u v w x y z "}},
				[][]interface{}{[]interface{}{"a\nb\nc"}},
				[][]interface{}{[]interface{}{testString1}},
			},
		},

		// int8: -128 to 127
		testQueryResults{
			query: "select :1 from dual",
			args: [][]interface{}{
				[]interface{}{int16(-128)},
				[]interface{}{int16(-1)},
				[]interface{}{int16(0)},
				[]interface{}{int16(1)},
				[]interface{}{int16(127)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(-128)}},
				[][]interface{}{[]interface{}{int64(-1)}},
				[][]interface{}{[]interface{}{int64(0)}},
				[][]interface{}{[]interface{}{int64(1)}},
				[][]interface{}{[]interface{}{int64(127)}},
			},
		},
		// int16: -32768 to 32767
		testQueryResults{
			query: "select :1 from dual",
			args: [][]interface{}{
				[]interface{}{int16(-32768)},
				[]interface{}{int16(-128)},
				[]interface{}{int16(-1)},
				[]interface{}{int16(0)},
				[]interface{}{int16(1)},
				[]interface{}{int16(127)},
				[]interface{}{int16(32767)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(-32768)}},
				[][]interface{}{[]interface{}{int64(-128)}},
				[][]interface{}{[]interface{}{int64(-1)}},
				[][]interface{}{[]interface{}{int64(0)}},
				[][]interface{}{[]interface{}{int64(1)}},
				[][]interface{}{[]interface{}{int64(127)}},
				[][]interface{}{[]interface{}{int64(32767)}},
			},
		},
		// int32: -2147483648 to 2147483647
		testQueryResults{
			query: "select :1 from dual",
			args: [][]interface{}{
				[]interface{}{int32(-2147483648)},
				[]interface{}{int32(-32768)},
				[]interface{}{int32(-128)},
				[]interface{}{int32(-1)},
				[]interface{}{int32(0)},
				[]interface{}{int32(1)},
				[]interface{}{int32(127)},
				[]interface{}{int32(32767)},
				[]interface{}{int32(2147483647)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(-2147483648)}},
				[][]interface{}{[]interface{}{int64(-32768)}},
				[][]interface{}{[]interface{}{int64(-128)}},
				[][]interface{}{[]interface{}{int64(-1)}},
				[][]interface{}{[]interface{}{int64(0)}},
				[][]interface{}{[]interface{}{int64(1)}},
				[][]interface{}{[]interface{}{int64(127)}},
				[][]interface{}{[]interface{}{int64(32767)}},
				[][]interface{}{[]interface{}{int64(2147483647)}},
			},
		},
		// int64: -9223372036854775808 to 9223372036854775807
		testQueryResults{
			query: "select :1 from dual",
			args: [][]interface{}{
				[]interface{}{int64(-9223372036854775808)},
				[]interface{}{int64(-2147483648)},
				[]interface{}{int64(-32768)},
				[]interface{}{int64(-128)},
				[]interface{}{int64(-1)},
				[]interface{}{int64(0)},
				[]interface{}{int64(1)},
				[]interface{}{int64(127)},
				[]interface{}{int64(32767)},
				[]interface{}{int64(2147483647)},
				[]interface{}{int64(9223372036854775807)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(-9223372036854775808)}},
				[][]interface{}{[]interface{}{int64(-2147483648)}},
				[][]interface{}{[]interface{}{int64(-32768)}},
				[][]interface{}{[]interface{}{int64(-128)}},
				[][]interface{}{[]interface{}{int64(-1)}},
				[][]interface{}{[]interface{}{int64(0)}},
				[][]interface{}{[]interface{}{int64(1)}},
				[][]interface{}{[]interface{}{int64(127)}},
				[][]interface{}{[]interface{}{int64(32767)}},
				[][]interface{}{[]interface{}{int64(2147483647)}},
				[][]interface{}{[]interface{}{int64(9223372036854775807)}},
			},
		},

		// uint8: 0 to 255
		testQueryResults{
			query: "select :1 from dual",
			args: [][]interface{}{
				[]interface{}{uint32(0)},
				[]interface{}{uint32(1)},
				[]interface{}{uint32(127)},
				[]interface{}{uint32(255)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(0)}},
				[][]interface{}{[]interface{}{int64(1)}},
				[][]interface{}{[]interface{}{int64(127)}},
				[][]interface{}{[]interface{}{int64(255)}},
			},
		},
		// uint16: 0 to 65535
		testQueryResults{
			query: "select :1 from dual",
			args: [][]interface{}{
				[]interface{}{uint32(0)},
				[]interface{}{uint32(1)},
				[]interface{}{uint32(127)},
				[]interface{}{uint32(32767)},
				[]interface{}{uint32(65535)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(0)}},
				[][]interface{}{[]interface{}{int64(1)}},
				[][]interface{}{[]interface{}{int64(127)}},
				[][]interface{}{[]interface{}{int64(32767)}},
				[][]interface{}{[]interface{}{int64(65535)}},
			},
		},
		// uint32: 0 to 4294967295
		testQueryResults{
			query: "select :1 from dual",
			args: [][]interface{}{
				[]interface{}{uint32(0)},
				[]interface{}{uint32(1)},
				[]interface{}{uint32(127)},
				[]interface{}{uint32(32767)},
				[]interface{}{uint32(2147483647)},
				[]interface{}{uint32(4294967295)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(0)}},
				[][]interface{}{[]interface{}{int64(1)}},
				[][]interface{}{[]interface{}{int64(127)}},
				[][]interface{}{[]interface{}{int64(32767)}},
				[][]interface{}{[]interface{}{int64(2147483647)}},
				[][]interface{}{[]interface{}{int64(4294967295)}},
			},
		},
		// uint64: 0 to 18446744073709551615
		testQueryResults{
			query: "select :1 from dual",
			args: [][]interface{}{
				[]interface{}{uint64(0)},
				[]interface{}{uint64(1)},
				[]interface{}{uint64(127)},
				[]interface{}{uint64(32767)},
				[]interface{}{uint64(2147483647)},
				[]interface{}{uint64(9223372036854775807)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(0)}},
				[][]interface{}{[]interface{}{int64(1)}},
				[][]interface{}{[]interface{}{int64(127)}},
				[][]interface{}{[]interface{}{int64(32767)}},
				[][]interface{}{[]interface{}{int64(2147483647)}},
				[][]interface{}{[]interface{}{int64(9223372036854775807)}},
			},
		},

		// byte
		testQueryResults{
			query: "select :1 from dual",
			args: [][]interface{}{
				[]interface{}{byte('a')},
				[]interface{}{byte('z')},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(97)}},
				[][]interface{}{[]interface{}{int64(122)}},
			},
		},

		// rune
		testQueryResults{
			query: "select :1 from dual",
			args: [][]interface{}{
				[]interface{}{'a'},
				[]interface{}{'z'},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{int64(97)}},
				[][]interface{}{[]interface{}{int64(122)}},
			},
		},

		// float32
		testQueryResults{
			query: "select :1 from dual",
			args: [][]interface{}{
				[]interface{}{float32(-9223372036854775808)},
				[]interface{}{float32(-2147483648)},
				[]interface{}{float32(-32767.123046875)},
				[]interface{}{float32(-32767)},
				[]interface{}{float32(-128.1234588623046875)},
				[]interface{}{float32(-128)},
				[]interface{}{float32(-1.12345683574676513671875)},
				[]interface{}{float32(-1)},
				[]interface{}{float32(-0.12345679104328155517578125)},
				[]interface{}{float32(0)},
				[]interface{}{float32(0.12345679104328155517578125)},
				[]interface{}{float32(1)},
				[]interface{}{float32(1.12345683574676513671875)},
				[]interface{}{float32(128)},
				[]interface{}{float32(128.1234588623046875)},
				[]interface{}{float32(32767)},
				[]interface{}{float32(32767.123046875)},
				[]interface{}{float32(2147483648)},
				[]interface{}{float32(9223372036854775808)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{float64(-9223372036854775808)}},
				[][]interface{}{[]interface{}{float64(-2147483648)}},
				[][]interface{}{[]interface{}{float64(-32767.123046875)}},
				[][]interface{}{[]interface{}{float64(-32767)}},
				[][]interface{}{[]interface{}{float64(-128.1234588623046875)}},
				[][]interface{}{[]interface{}{float64(-128)}},
				[][]interface{}{[]interface{}{float64(-1.12345683574676513671875)}},
				[][]interface{}{[]interface{}{float64(-1)}},
				[][]interface{}{[]interface{}{float64(-0.12345679104328155517578125)}},
				[][]interface{}{[]interface{}{float64(0)}},
				[][]interface{}{[]interface{}{float64(0.12345679104328155517578125)}},
				[][]interface{}{[]interface{}{float64(1)}},
				[][]interface{}{[]interface{}{float64(1.12345683574676513671875)}},
				[][]interface{}{[]interface{}{float64(128)}},
				[][]interface{}{[]interface{}{float64(128.1234588623046875)}},
				[][]interface{}{[]interface{}{float64(32767)}},
				[][]interface{}{[]interface{}{float64(32767.123046875)}},
				[][]interface{}{[]interface{}{float64(2147483648)}},
				[][]interface{}{[]interface{}{float64(9223372036854775808)}},
			},
		},
		// float64
		testQueryResults{
			query: "select :1 from dual",
			args: [][]interface{}{
				[]interface{}{float64(-9223372036854775808)},
				[]interface{}{float64(-2147483648)},
				[]interface{}{float64(-32767.123046875)},
				[]interface{}{float64(-32767)},
				[]interface{}{float64(-128.1234588623046875)},
				[]interface{}{float64(-128)},
				[]interface{}{float64(-1.12345683574676513671875)},
				[]interface{}{float64(-1)},
				[]interface{}{float64(-0.12345679104328155517578125)},
				[]interface{}{float64(0)},
				[]interface{}{float64(0.12345679104328155517578125)},
				[]interface{}{float64(1)},
				[]interface{}{float64(1.12345683574676513671875)},
				[]interface{}{float64(128)},
				[]interface{}{float64(128.1234588623046875)},
				[]interface{}{float64(32767)},
				[]interface{}{float64(32767.123046875)},
				[]interface{}{float64(2147483648)},
				[]interface{}{float64(9223372036854775808)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{float64(-9223372036854775808)}},
				[][]interface{}{[]interface{}{float64(-2147483648)}},
				[][]interface{}{[]interface{}{float64(-32767.123046875)}},
				[][]interface{}{[]interface{}{float64(-32767)}},
				[][]interface{}{[]interface{}{float64(-128.1234588623046875)}},
				[][]interface{}{[]interface{}{float64(-128)}},
				[][]interface{}{[]interface{}{float64(-1.12345683574676513671875)}},
				[][]interface{}{[]interface{}{float64(-1)}},
				[][]interface{}{[]interface{}{float64(-0.12345679104328155517578125)}},
				[][]interface{}{[]interface{}{float64(0)}},
				[][]interface{}{[]interface{}{float64(0.12345679104328155517578125)}},
				[][]interface{}{[]interface{}{float64(1)}},
				[][]interface{}{[]interface{}{float64(1.12345683574676513671875)}},
				[][]interface{}{[]interface{}{float64(128)}},
				[][]interface{}{[]interface{}{float64(128.1234588623046875)}},
				[][]interface{}{[]interface{}{float64(32767)}},
				[][]interface{}{[]interface{}{float64(32767.123046875)}},
				[][]interface{}{[]interface{}{float64(2147483648)}},
				[][]interface{}{[]interface{}{float64(9223372036854775808)}},
			},
		},

		// time
		testQueryResults{
			query: "select :1 from dual",
			args: [][]interface{}{
				[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, time.UTC)},
				[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocUTC)},
				[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocGMT)},
				[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocEST)},
				[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocMST)},
				[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocNZ)},
				[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)},
				[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocUTC)},
				[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocGMT)},
				[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocEST)},
				[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocMST)},
				// TOFIX: ORA-08192: Flashback Table operation is not allowed on fixed tables
				// []interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocNZ)},
				[]interface{}{time.Date(9998, 12, 31, 3, 4, 5, 123456789, time.UTC)},
				[]interface{}{time.Date(9998, 12, 31, 3, 4, 5, 123456789, testTimeLocUTC)},
				[]interface{}{time.Date(9998, 12, 31, 3, 4, 5, 123456789, testTimeLocGMT)},
				[]interface{}{time.Date(9998, 12, 31, 3, 4, 5, 123456789, testTimeLocEST)},
				[]interface{}{time.Date(9998, 12, 31, 3, 4, 5, 123456789, testTimeLocMST)},
				[]interface{}{time.Date(9998, 12, 31, 3, 4, 5, 123456789, testTimeLocNZ)},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, time.UTC)}},
				[][]interface{}{[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocUTC)}},
				[][]interface{}{[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocGMT)}},
				[][]interface{}{[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocEST)}},
				[][]interface{}{[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocMST)}},
				[][]interface{}{[]interface{}{time.Date(2006, 1, 2, 3, 4, 5, 123456789, testTimeLocNZ)}},
				[][]interface{}{[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)}},
				[][]interface{}{[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocUTC)}},
				[][]interface{}{[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocGMT)}},
				[][]interface{}{[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocEST)}},
				[][]interface{}{[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocMST)}},
				// TOFIX: ORA-08192: Flashback Table operation is not allowed on fixed tables
				// [][]interface{}{[]interface{}{time.Date(1, 1, 1, 0, 0, 0, 0, testTimeLocNZ)}},
				[][]interface{}{[]interface{}{time.Date(9998, 12, 31, 3, 4, 5, 123456789, time.UTC)}},
				[][]interface{}{[]interface{}{time.Date(9998, 12, 31, 3, 4, 5, 123456789, testTimeLocUTC)}},
				[][]interface{}{[]interface{}{time.Date(9998, 12, 31, 3, 4, 5, 123456789, testTimeLocGMT)}},
				[][]interface{}{[]interface{}{time.Date(9998, 12, 31, 3, 4, 5, 123456789, testTimeLocEST)}},
				[][]interface{}{[]interface{}{time.Date(9998, 12, 31, 3, 4, 5, 123456789, testTimeLocMST)}},
				[][]interface{}{[]interface{}{time.Date(9998, 12, 31, 3, 4, 5, 123456789, testTimeLocNZ)}},
			},
		},

		// []byte
		testQueryResults{
			query: "select :1 from dual",
			args: [][]interface{}{
				[]interface{}{[]byte{}},
				[]interface{}{[]byte{10}},
				[]interface{}{[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
				[]interface{}{[]byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}},
				[]interface{}{[]byte{245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255}},
				[]interface{}{testByteSlice1},
			},
			results: [][][]interface{}{
				[][]interface{}{[]interface{}{nil}},
				[][]interface{}{[]interface{}{[]byte{10}}},
				[][]interface{}{[]interface{}{[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}},
				[][]interface{}{[]interface{}{[]byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}}},
				[][]interface{}{[]interface{}{[]byte{245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255}}},
				[][]interface{}{[]interface{}{testByteSlice1}},
			},
		},
	}

	testRunQueryResults(t, queryResults)
}

// TestDestructiveString checks insert, select, update, and delete of string types
func TestDestructiveString(t *testing.T) {
	if TestDisableDatabase || TestDisableDestructive {
		t.SkipNow()
	}

	// https://ss64.com/ora/syntax-datatypes.html

	// VARCHAR2
	err := testExec(t, "create table VARCHAR2_"+TestTimeString+
		" ( A VARCHAR2(1), B VARCHAR2(2000), C VARCHAR2(4000) )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table VARCHAR2_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into VARCHAR2_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{"a", strings.Repeat("a", 2000), strings.Repeat("a", 4000)},
			[]interface{}{"b", strings.Repeat("b", 2000), strings.Repeat("b", 4000)},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults := []testQueryResults{
		testQueryResults{
			query: "select A, B, C from VARCHAR2_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"a", strings.Repeat("a", 2000), strings.Repeat("a", 4000)},
					[]interface{}{"b", strings.Repeat("b", 2000), strings.Repeat("b", 4000)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExec(t, "delete from VARCHAR2_"+TestTimeString+" where A = :1", []interface{}{"a"})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from VARCHAR2_" + TestTimeString,
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"b", strings.Repeat("b", 2000), strings.Repeat("b", 4000)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// NVARCHAR2
	err = testExec(t, "create table NVARCHAR2_"+TestTimeString+
		" ( A NVARCHAR2(1), B NVARCHAR2(1000), C NVARCHAR2(2000) )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table NVARCHAR2_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into NVARCHAR2_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{"a", strings.Repeat("a", 1000), strings.Repeat("a", 2000)},
			[]interface{}{"b", strings.Repeat("b", 1000), strings.Repeat("b", 2000)},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NVARCHAR2_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"a", strings.Repeat("a", 1000), strings.Repeat("a", 2000)},
					[]interface{}{"b", strings.Repeat("b", 1000), strings.Repeat("b", 2000)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExec(t, "delete from NVARCHAR2_"+TestTimeString+" where A = :1", []interface{}{"a"})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NVARCHAR2_" + TestTimeString,
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"b", strings.Repeat("b", 1000), strings.Repeat("b", 2000)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// CHAR
	err = testExec(t, "create table CHAR_"+TestTimeString+
		" ( A CHAR(1), B CHAR(1000), C CHAR(2000) )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table CHAR_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into CHAR_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{"a", strings.Repeat("a", 1000), strings.Repeat("a", 2000)},
			[]interface{}{"b", strings.Repeat("b", 1000), strings.Repeat("b", 2000)},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from CHAR_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"a", strings.Repeat("a", 1000), strings.Repeat("a", 2000)},
					[]interface{}{"b", strings.Repeat("b", 1000), strings.Repeat("b", 2000)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExec(t, "delete from CHAR_"+TestTimeString+" where A = :1", []interface{}{"a"})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from CHAR_" + TestTimeString,
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"b", strings.Repeat("b", 1000), strings.Repeat("b", 2000)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExec(t, "delete from CHAR_"+TestTimeString+" where A = :1", []interface{}{"b"})
	if err != nil {
		t.Error("delete error:", err)
	}

	err = testExecRows(t, "insert into CHAR_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{"a", strings.Repeat("a", 100), strings.Repeat("a", 200)},
			[]interface{}{"b", strings.Repeat("b", 100), strings.Repeat("b", 200)},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from CHAR_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"a", strings.Repeat("a", 100) + strings.Repeat(" ", 900), strings.Repeat("a", 200) + strings.Repeat(" ", 1800)},
					[]interface{}{"b", strings.Repeat("b", 100) + strings.Repeat(" ", 900), strings.Repeat("b", 200) + strings.Repeat(" ", 1800)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExec(t, "delete from CHAR_"+TestTimeString+" where A = :1", []interface{}{"a"})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from CHAR_" + TestTimeString,
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"b", strings.Repeat("b", 100) + strings.Repeat(" ", 900), strings.Repeat("b", 200) + strings.Repeat(" ", 1800)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// NCHAR
	err = testExec(t, "create table NCHAR_"+TestTimeString+
		" ( A NCHAR(1), B NCHAR(500), C NCHAR(1000) )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table NCHAR_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into NCHAR_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{"a", strings.Repeat("a", 500), strings.Repeat("a", 1000)},
			[]interface{}{"b", strings.Repeat("b", 500), strings.Repeat("b", 1000)},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NCHAR_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"a", strings.Repeat("a", 500), strings.Repeat("a", 1000)},
					[]interface{}{"b", strings.Repeat("b", 500), strings.Repeat("b", 1000)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExec(t, "delete from NCHAR_"+TestTimeString+" where A = :1", []interface{}{"a"})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NCHAR_" + TestTimeString,
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"b", strings.Repeat("b", 500), strings.Repeat("b", 1000)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExec(t, "delete from NCHAR_"+TestTimeString+" where A = :1", []interface{}{"b"})
	if err != nil {
		t.Error("delete error:", err)
	}

	err = testExecRows(t, "insert into NCHAR_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{"a", strings.Repeat("a", 100), strings.Repeat("a", 200)},
			[]interface{}{"b", strings.Repeat("b", 100), strings.Repeat("b", 200)},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NCHAR_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"a", strings.Repeat("a", 100) + strings.Repeat(" ", 400), strings.Repeat("a", 200) + strings.Repeat(" ", 800)},
					[]interface{}{"b", strings.Repeat("b", 100) + strings.Repeat(" ", 400), strings.Repeat("b", 200) + strings.Repeat(" ", 800)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExec(t, "delete from NCHAR_"+TestTimeString+" where A = :1", []interface{}{"a"})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NCHAR_" + TestTimeString,
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"b", strings.Repeat("b", 100) + strings.Repeat(" ", 400), strings.Repeat("b", 200) + strings.Repeat(" ", 800)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// RAW
	err = testExec(t, "create table RAW_"+TestTimeString+
		" ( A RAW(1), B RAW(1000), C RAW(2000) )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table RAW_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into RAW_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{[]byte{}, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, []byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}},
			[]interface{}{[]byte{10}, []byte{245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255}, testByteSlice1},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from RAW_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{[]byte{10}, []byte{245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255}, testByteSlice1},
					[]interface{}{nil, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, []byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExec(t, "delete from RAW_"+TestTimeString+" where A = :1", []interface{}{[]byte{10}})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from RAW_" + TestTimeString,
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{nil, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, []byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// ROWID
	ctx, cancel := context.WithTimeout(context.Background(), TestContextTimeout)
	stmt, err := TestDB.PrepareContext(ctx, "select ROWID from RAW_"+TestTimeString)
	cancel()
	if err != nil {
		t.Fatal("prepare error:", err)
	}

	var result [][]interface{}
	result, err = testGetRows(t, stmt, nil)
	if err != nil {
		t.Fatal("get rows error:", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result) < 1 {
		t.Fatal("len result less than 1")
	}
	if len(result[0]) < 1 {
		t.Fatal("len result[0] less than 1")
	}
	data, ok := result[0][0].(string)
	if !ok {
		t.Fatal("result not string")
	}
	if len(data) != 18 {
		t.Fatal("result len not equal to 18:", len(data))
	}

	// CLOB
	err = testExec(t, "create table CLOB_"+TestTimeString+
		" ( A VARCHAR2(100), B CLOB, C CLOB )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table CLOB_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into CLOB_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{"a", strings.Repeat("a", 2000), strings.Repeat("a", 4000)},
			[]interface{}{"b", strings.Repeat("b", 2000), strings.Repeat("b", 4000)},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from CLOB_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"a", strings.Repeat("a", 2000), strings.Repeat("a", 4000)},
					[]interface{}{"b", strings.Repeat("b", 2000), strings.Repeat("b", 4000)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExec(t, "delete from CLOB_"+TestTimeString+" where A = :1", []interface{}{"a"})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from CLOB_" + TestTimeString,
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"b", strings.Repeat("b", 2000), strings.Repeat("b", 4000)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// NCLOB
	err = testExec(t, "create table NCLOB_"+TestTimeString+
		" ( A VARCHAR2(100), B NCLOB, C NCLOB )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table NCLOB_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into NCLOB_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{"a", strings.Repeat("a", 2000), strings.Repeat("a", 4000)},
			[]interface{}{"b", strings.Repeat("b", 2000), strings.Repeat("b", 4000)},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NCLOB_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"a", strings.Repeat("a", 2000), strings.Repeat("a", 4000)},
					[]interface{}{"b", strings.Repeat("b", 2000), strings.Repeat("b", 4000)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExec(t, "delete from NCLOB_"+TestTimeString+" where A = :1", []interface{}{"a"})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NCLOB_" + TestTimeString,
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"b", strings.Repeat("b", 2000), strings.Repeat("b", 4000)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// BLOB
	err = testExec(t, "create table BLOB_"+TestTimeString+
		" ( A VARCHAR2(100), B BLOB, C BLOB )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table BLOB_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into BLOB_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{"a", []byte{}, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
			[]interface{}{"b", []byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}, []byte{245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255}},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from BLOB_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"a", nil, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
					[]interface{}{"b", []byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}, []byte{245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255}},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExec(t, "delete from BLOB_"+TestTimeString+" where A = :1", []interface{}{"a"})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from BLOB_" + TestTimeString,
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{"b", []byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}, []byte{245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255}},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

}

// TestDestructiveNumber checks insert, select, update, and delete of number types
func TestDestructiveNumber(t *testing.T) {
	if TestDisableDatabase || TestDisableDestructive {
		t.SkipNow()
	}

	// https://ss64.com/ora/syntax-datatypes.html

	// NUMBER negative
	err := testExec(t, "create table NUMBER_"+TestTimeString+
		" ( A NUMBER(10,2), B NUMBER(20,4), C NUMBER(38,8) )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table NUMBER_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into NUMBER_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{-9999999.99, -999999999999999.9999, -9999999999999999999999999.99999999},
			[]interface{}{-21474836, -2147483648, -2147483648},
			[]interface{}{-1234567, -123456792, -123456792},
			[]interface{}{-1.98, -1.9873, -1.98730468},
			[]interface{}{-1, -1, -1},
			[]interface{}{-0.76, -0.7617, -0.76171875},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults := []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NUMBER_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(-21474836), float64(-2147483648), float64(-2147483648)},
					[]interface{}{float64(-9999999.99), float64(-999999999999999.9999), float64(-9999999999999999999999999.99999999)},
					[]interface{}{float64(-1234567), float64(-123456792), float64(-123456792)},
					[]interface{}{float64(-1.98), float64(-1.9873), float64(-1.98730468)},
					[]interface{}{float64(-1), float64(-1), float64(-1)},
					[]interface{}{float64(-0.76), float64(-0.7617), float64(-0.76171875)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from NUMBER_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{-21474836},
			[]interface{}{-9999999.99},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NUMBER_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(-1234567), float64(-123456792), float64(-123456792)},
					[]interface{}{float64(-1.98), float64(-1.9873), float64(-1.98730468)},
					[]interface{}{float64(-1), float64(-1), float64(-1)},
					[]interface{}{float64(-0.76), float64(-0.7617), float64(-0.76171875)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// test truncate
	err = testExec(t, "truncate table NUMBER_"+TestTimeString, nil)
	if err != nil {
		t.Error("truncate error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NUMBER_" + TestTimeString,
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// NUMBER positive
	err = testExecRows(t, "insert into NUMBER_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{0, 0, 0},
			[]interface{}{0.76, 0.7617, 0.76171875},
			[]interface{}{1, 1, 1},
			[]interface{}{1.98, 1.9873, 1.98730468},
			[]interface{}{12345679, 123456792, 123456792},
			[]interface{}{21474836, 2147483647, 2147483647},
			[]interface{}{9999999.99, 999999999999999.9999, 99999999999999999999999999.99999999},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NUMBER_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(0), float64(0), float64(0)},
					[]interface{}{float64(0.76), float64(0.7617), float64(0.76171875)},
					[]interface{}{float64(1), float64(1), float64(1)},
					[]interface{}{float64(1.98), float64(1.9873), float64(1.98730468)},
					[]interface{}{float64(9999999.99), float64(999999999999999.9999), float64(99999999999999999999999999.99999999)},
					[]interface{}{float64(12345679), float64(123456792), float64(123456792)},
					[]interface{}{float64(21474836), float64(2147483647), float64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from NUMBER_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{0},
			[]interface{}{1},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NUMBER_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(0.76), float64(0.7617), float64(0.76171875)},
					[]interface{}{float64(1.98), float64(1.9873), float64(1.98730468)},
					[]interface{}{float64(9999999.99), float64(999999999999999.9999), float64(99999999999999999999999999.99999999)},
					[]interface{}{float64(12345679), float64(123456792), float64(123456792)},
					[]interface{}{float64(21474836), float64(2147483647), float64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// DEC negative
	err = testExec(t, "create table DEC_"+TestTimeString+
		" ( A DEC(10,2), B DEC(20,4), C DEC(38,8) )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table DEC_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into DEC_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{-9999999.99, -999999999999999.9999, -9999999999999999999999999.99999999},
			[]interface{}{-21474836, -2147483648, -2147483648},
			[]interface{}{-1234567, -123456792, -123456792},
			[]interface{}{-1.98, -1.9873, -1.98730468},
			[]interface{}{-1, -1, -1},
			[]interface{}{-0.76, -0.7617, -0.76171875},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from DEC_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(-21474836), float64(-2147483648), float64(-2147483648)},
					[]interface{}{float64(-9999999.99), float64(-999999999999999.9999), float64(-9999999999999999999999999.99999999)},
					[]interface{}{float64(-1234567), float64(-123456792), float64(-123456792)},
					[]interface{}{float64(-1.98), float64(-1.9873), float64(-1.98730468)},
					[]interface{}{float64(-1), float64(-1), float64(-1)},
					[]interface{}{float64(-0.76), float64(-0.7617), float64(-0.76171875)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from DEC_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{-21474836},
			[]interface{}{-9999999.99},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from DEC_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(-1234567), float64(-123456792), float64(-123456792)},
					[]interface{}{float64(-1.98), float64(-1.9873), float64(-1.98730468)},
					[]interface{}{float64(-1), float64(-1), float64(-1)},
					[]interface{}{float64(-0.76), float64(-0.7617), float64(-0.76171875)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// DEC positive
	err = testExec(t, "truncate table DEC_"+TestTimeString, nil)
	if err != nil {
		t.Error("truncate error:", err)
	}

	err = testExecRows(t, "insert into DEC_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{0, 0, 0},
			[]interface{}{0.76, 0.7617, 0.76171875},
			[]interface{}{1, 1, 1},
			[]interface{}{1.98, 1.9873, 1.98730468},
			[]interface{}{12345679, 123456792, 123456792},
			[]interface{}{21474836, 2147483647, 2147483647},
			[]interface{}{9999999.99, 999999999999999.9999, 99999999999999999999999999.99999999},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from DEC_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(0), float64(0), float64(0)},
					[]interface{}{float64(0.76), float64(0.7617), float64(0.76171875)},
					[]interface{}{float64(1), float64(1), float64(1)},
					[]interface{}{float64(1.98), float64(1.9873), float64(1.98730468)},
					[]interface{}{float64(9999999.99), float64(999999999999999.9999), float64(99999999999999999999999999.99999999)},
					[]interface{}{float64(12345679), float64(123456792), float64(123456792)},
					[]interface{}{float64(21474836), float64(2147483647), float64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from DEC_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{0},
			[]interface{}{1},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from DEC_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(0.76), float64(0.7617), float64(0.76171875)},
					[]interface{}{float64(1.98), float64(1.9873), float64(1.98730468)},
					[]interface{}{float64(9999999.99), float64(999999999999999.9999), float64(99999999999999999999999999.99999999)},
					[]interface{}{float64(12345679), float64(123456792), float64(123456792)},
					[]interface{}{float64(21474836), float64(2147483647), float64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// DECIMAL negative
	err = testExec(t, "create table DECIMAL_"+TestTimeString+
		" ( A DECIMAL(10,2), B DECIMAL(20,4), C DECIMAL(38,8) )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table DECIMAL_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into DECIMAL_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{-9999999.99, -999999999999999.9999, -9999999999999999999999999.99999999},
			[]interface{}{-21474836, -2147483648, -2147483648},
			[]interface{}{-1234567, -123456792, -123456792},
			[]interface{}{-1.98, -1.9873, -1.98730468},
			[]interface{}{-1, -1, -1},
			[]interface{}{-0.76, -0.7617, -0.76171875},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from DECIMAL_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(-21474836), float64(-2147483648), float64(-2147483648)},
					[]interface{}{float64(-9999999.99), float64(-999999999999999.9999), float64(-9999999999999999999999999.99999999)},
					[]interface{}{float64(-1234567), float64(-123456792), float64(-123456792)},
					[]interface{}{float64(-1.98), float64(-1.9873), float64(-1.98730468)},
					[]interface{}{float64(-1), float64(-1), float64(-1)},
					[]interface{}{float64(-0.76), float64(-0.7617), float64(-0.76171875)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from DECIMAL_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{-21474836},
			[]interface{}{-9999999.99},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from DECIMAL_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(-1234567), float64(-123456792), float64(-123456792)},
					[]interface{}{float64(-1.98), float64(-1.9873), float64(-1.98730468)},
					[]interface{}{float64(-1), float64(-1), float64(-1)},
					[]interface{}{float64(-0.76), float64(-0.7617), float64(-0.76171875)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// DECIMAL positive
	err = testExec(t, "truncate table DECIMAL_"+TestTimeString, nil)
	if err != nil {
		t.Error("truncate error:", err)
	}

	err = testExecRows(t, "insert into DECIMAL_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{0, 0, 0},
			[]interface{}{0.76, 0.7617, 0.76171875},
			[]interface{}{1, 1, 1},
			[]interface{}{1.98, 1.9873, 1.98730468},
			[]interface{}{12345679, 123456792, 123456792},
			[]interface{}{21474836, 2147483647, 2147483647},
			[]interface{}{9999999.99, 999999999999999.9999, 99999999999999999999999999.99999999},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from DECIMAL_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(0), float64(0), float64(0)},
					[]interface{}{float64(0.76), float64(0.7617), float64(0.76171875)},
					[]interface{}{float64(1), float64(1), float64(1)},
					[]interface{}{float64(1.98), float64(1.9873), float64(1.98730468)},
					[]interface{}{float64(9999999.99), float64(999999999999999.9999), float64(99999999999999999999999999.99999999)},
					[]interface{}{float64(12345679), float64(123456792), float64(123456792)},
					[]interface{}{float64(21474836), float64(2147483647), float64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from DECIMAL_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{0},
			[]interface{}{1},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from DECIMAL_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(0.76), float64(0.7617), float64(0.76171875)},
					[]interface{}{float64(1.98), float64(1.9873), float64(1.98730468)},
					[]interface{}{float64(9999999.99), float64(999999999999999.9999), float64(99999999999999999999999999.99999999)},
					[]interface{}{float64(12345679), float64(123456792), float64(123456792)},
					[]interface{}{float64(21474836), float64(2147483647), float64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// NUMERIC negative
	err = testExec(t, "create table NUMERIC_"+TestTimeString+
		" ( A NUMERIC(10,2), B NUMERIC(20,4), C NUMERIC(38,8) )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table NUMERIC_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into NUMERIC_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{-9999999.99, -999999999999999.9999, -9999999999999999999999999.99999999},
			[]interface{}{-21474836, -2147483648, -2147483648},
			[]interface{}{-1234567, -123456792, -123456792},
			[]interface{}{-1.98, -1.9873, -1.98730468},
			[]interface{}{-1, -1, -1},
			[]interface{}{-0.76, -0.7617, -0.76171875},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NUMERIC_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(-21474836), float64(-2147483648), float64(-2147483648)},
					[]interface{}{float64(-9999999.99), float64(-999999999999999.9999), float64(-9999999999999999999999999.99999999)},
					[]interface{}{float64(-1234567), float64(-123456792), float64(-123456792)},
					[]interface{}{float64(-1.98), float64(-1.9873), float64(-1.98730468)},
					[]interface{}{float64(-1), float64(-1), float64(-1)},
					[]interface{}{float64(-0.76), float64(-0.7617), float64(-0.76171875)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from NUMERIC_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{-21474836},
			[]interface{}{-9999999.99},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NUMERIC_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(-1234567), float64(-123456792), float64(-123456792)},
					[]interface{}{float64(-1.98), float64(-1.9873), float64(-1.98730468)},
					[]interface{}{float64(-1), float64(-1), float64(-1)},
					[]interface{}{float64(-0.76), float64(-0.7617), float64(-0.76171875)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// NUMERIC positive
	err = testExec(t, "truncate table NUMERIC_"+TestTimeString, nil)
	if err != nil {
		t.Error("truncate error:", err)
	}

	err = testExecRows(t, "insert into NUMERIC_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{0, 0, 0},
			[]interface{}{0.76, 0.7617, 0.76171875},
			[]interface{}{1, 1, 1},
			[]interface{}{1.98, 1.9873, 1.98730468},
			[]interface{}{12345679, 123456792, 123456792},
			[]interface{}{21474836, 2147483647, 2147483647},
			[]interface{}{9999999.99, 999999999999999.9999, 99999999999999999999999999.99999999},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NUMERIC_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(0), float64(0), float64(0)},
					[]interface{}{float64(0.76), float64(0.7617), float64(0.76171875)},
					[]interface{}{float64(1), float64(1), float64(1)},
					[]interface{}{float64(1.98), float64(1.9873), float64(1.98730468)},
					[]interface{}{float64(9999999.99), float64(999999999999999.9999), float64(99999999999999999999999999.99999999)},
					[]interface{}{float64(12345679), float64(123456792), float64(123456792)},
					[]interface{}{float64(21474836), float64(2147483647), float64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from NUMERIC_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{0},
			[]interface{}{1},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from NUMERIC_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(0.76), float64(0.7617), float64(0.76171875)},
					[]interface{}{float64(1.98), float64(1.9873), float64(1.98730468)},
					[]interface{}{float64(9999999.99), float64(999999999999999.9999), float64(99999999999999999999999999.99999999)},
					[]interface{}{float64(12345679), float64(123456792), float64(123456792)},
					[]interface{}{float64(21474836), float64(2147483647), float64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// FLOAT negative
	err = testExec(t, "create table FLOAT_"+TestTimeString+
		" ( A FLOAT(28), B FLOAT(32), C FLOAT(38) )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table FLOAT_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into FLOAT_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{-9999999.99, -999999999999999.9999, -9999999999999999999999999.99999999},
			[]interface{}{-21474836, -2147483648, -2147483648},
			[]interface{}{-1234567, -123456792, -123456792},
			[]interface{}{-1.98, -1.9873, -1.98730468},
			[]interface{}{-1, -1, -1},
			[]interface{}{-0.76, -0.7617, -0.76171875},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from FLOAT_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(-21474836), float64(-2147483648), float64(-2147483648)},
					[]interface{}{float64(-9999999.99), float64(-999999999999999.9999), float64(-9999999999999999999999999.99999999)},
					[]interface{}{float64(-1234567), float64(-123456792), float64(-123456792)},
					[]interface{}{float64(-1.98), float64(-1.9873), float64(-1.98730468)},
					[]interface{}{float64(-1), float64(-1), float64(-1)},
					[]interface{}{float64(-0.76), float64(-0.7617), float64(-0.76171875)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from FLOAT_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{-21474836},
			[]interface{}{-9999999.99},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from FLOAT_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(-1234567), float64(-123456792), float64(-123456792)},
					[]interface{}{float64(-1.98), float64(-1.9873), float64(-1.98730468)},
					[]interface{}{float64(-1), float64(-1), float64(-1)},
					[]interface{}{float64(-0.76), float64(-0.7617), float64(-0.76171875)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// FLOAT positive
	err = testExec(t, "truncate table FLOAT_"+TestTimeString, nil)
	if err != nil {
		t.Error("truncate error:", err)
	}

	err = testExecRows(t, "insert into FLOAT_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{0, 0, 0},
			[]interface{}{0.76, 0.7617, 0.76171875},
			[]interface{}{1, 1, 1},
			[]interface{}{1.98, 1.9873, 1.98730468},
			[]interface{}{12345679, 123456792, 123456792},
			[]interface{}{21474836, 2147483647, 2147483647},
			[]interface{}{9999999.99, 999999999999999.9999, 99999999999999999999999999.99999999},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from FLOAT_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(0), float64(0), float64(0)},
					[]interface{}{float64(0.76), float64(0.7617), float64(0.76171875)},
					[]interface{}{float64(1), float64(1), float64(1)},
					[]interface{}{float64(1.98), float64(1.9873), float64(1.98730468)},
					[]interface{}{float64(9999999.99), float64(999999999999999.9999), float64(99999999999999999999999999.99999999)},
					[]interface{}{float64(12345679), float64(123456792), float64(123456792)},
					[]interface{}{float64(21474836), float64(2147483647), float64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from FLOAT_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{0},
			[]interface{}{1},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from FLOAT_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(0.76), float64(0.7617), float64(0.76171875)},
					[]interface{}{float64(1.98), float64(1.9873), float64(1.98730468)},
					[]interface{}{float64(9999999.99), float64(999999999999999.9999), float64(99999999999999999999999999.99999999)},
					[]interface{}{float64(12345679), float64(123456792), float64(123456792)},
					[]interface{}{float64(21474836), float64(2147483647), float64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// INTEGER negative
	err = testExec(t, "create table INTEGER_"+TestTimeString+
		" ( A INTEGER, B INTEGER, C INTEGER )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table INTEGER_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into INTEGER_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{-9999999.99, -999999999999999.9999, -999999999999999.99999999},
			[]interface{}{-21474836, -2147483648, -2147483648},
			[]interface{}{-1234567, -123456792, -123456792},
			[]interface{}{-1.98, -1.9873, -1.98730468},
			[]interface{}{-1, -1, -1},
			[]interface{}{-0.76, -0.7617, -0.76171875},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from INTEGER_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(-21474836), int64(-2147483648), int64(-2147483648)},
					[]interface{}{int64(-10000000), int64(-1000000000000000), int64(-1000000000000000)},
					[]interface{}{int64(-1234567), int64(-123456792), int64(-123456792)},
					[]interface{}{int64(-2), int64(-2), int64(-2)},
					[]interface{}{int64(-1), int64(-1), int64(-1)},
					[]interface{}{int64(-1), int64(-1), int64(-1)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from INTEGER_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{-21474836},
			[]interface{}{-10000000},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from INTEGER_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(-1234567), int64(-123456792), int64(-123456792)},
					[]interface{}{int64(-2), int64(-2), int64(-2)},
					[]interface{}{int64(-1), int64(-1), int64(-1)},
					[]interface{}{int64(-1), int64(-1), int64(-1)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// INTEGER positive
	err = testExec(t, "truncate table INTEGER_"+TestTimeString, nil)
	if err != nil {
		t.Error("truncate error:", err)
	}

	err = testExecRows(t, "insert into INTEGER_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{0, 0, 0},
			[]interface{}{0.76, 0.7617, 0.76171875},
			[]interface{}{1, 1, 1},
			[]interface{}{1.98, 1.9873, 1.98730468},
			[]interface{}{12345679, 123456792, 123456792},
			[]interface{}{21474836, 2147483647, 2147483647},
			[]interface{}{9999999.99, 999999999999999.9999, 999999999999999.99999999},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from INTEGER_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(0), int64(0), int64(0)},
					[]interface{}{int64(1), int64(1), int64(1)},
					[]interface{}{int64(1), int64(1), int64(1)},
					[]interface{}{int64(2), int64(2), int64(2)},
					[]interface{}{int64(10000000), int64(1000000000000000), int64(1000000000000000)},
					[]interface{}{int64(12345679), int64(123456792), int64(123456792)},
					[]interface{}{int64(21474836), int64(2147483647), int64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from INTEGER_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{10000000},
			[]interface{}{12345679},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from INTEGER_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(0), int64(0), int64(0)},
					[]interface{}{int64(1), int64(1), int64(1)},
					[]interface{}{int64(1), int64(1), int64(1)},
					[]interface{}{int64(2), int64(2), int64(2)},
					[]interface{}{int64(21474836), int64(2147483647), int64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// INT negative
	err = testExec(t, "create table INT_"+TestTimeString+
		" ( A INT, B INT, C INT )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table INT_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into INT_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{-9999999.99, -999999999999999.9999, -999999999999999.99999999},
			[]interface{}{-21474836, -2147483648, -2147483648},
			[]interface{}{-1234567, -123456792, -123456792},
			[]interface{}{-1.98, -1.9873, -1.98730468},
			[]interface{}{-1, -1, -1},
			[]interface{}{-0.76, -0.7617, -0.76171875},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from INT_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(-21474836), int64(-2147483648), int64(-2147483648)},
					[]interface{}{int64(-10000000), int64(-1000000000000000), int64(-1000000000000000)},
					[]interface{}{int64(-1234567), int64(-123456792), int64(-123456792)},
					[]interface{}{int64(-2), int64(-2), int64(-2)},
					[]interface{}{int64(-1), int64(-1), int64(-1)},
					[]interface{}{int64(-1), int64(-1), int64(-1)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from INT_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{-21474836},
			[]interface{}{-10000000},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from INT_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(-1234567), int64(-123456792), int64(-123456792)},
					[]interface{}{int64(-2), int64(-2), int64(-2)},
					[]interface{}{int64(-1), int64(-1), int64(-1)},
					[]interface{}{int64(-1), int64(-1), int64(-1)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// INT positive
	err = testExec(t, "truncate table INT_"+TestTimeString, nil)
	if err != nil {
		t.Error("truncate error:", err)
	}

	err = testExecRows(t, "insert into INT_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{0, 0, 0},
			[]interface{}{0.76, 0.7617, 0.76171875},
			[]interface{}{1, 1, 1},
			[]interface{}{1.98, 1.9873, 1.98730468},
			[]interface{}{12345679, 123456792, 123456792},
			[]interface{}{21474836, 2147483647, 2147483647},
			[]interface{}{9999999.99, 999999999999999.9999, 999999999999999.99999999},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from INT_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(0), int64(0), int64(0)},
					[]interface{}{int64(1), int64(1), int64(1)},
					[]interface{}{int64(1), int64(1), int64(1)},
					[]interface{}{int64(2), int64(2), int64(2)},
					[]interface{}{int64(10000000), int64(1000000000000000), int64(1000000000000000)},
					[]interface{}{int64(12345679), int64(123456792), int64(123456792)},
					[]interface{}{int64(21474836), int64(2147483647), int64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from INT_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{10000000},
			[]interface{}{12345679},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from INT_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(0), int64(0), int64(0)},
					[]interface{}{int64(1), int64(1), int64(1)},
					[]interface{}{int64(1), int64(1), int64(1)},
					[]interface{}{int64(2), int64(2), int64(2)},
					[]interface{}{int64(21474836), int64(2147483647), int64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// SMALLINT negative
	err = testExec(t, "create table SMALLINT_"+TestTimeString+
		" ( A SMALLINT, B SMALLINT, C SMALLINT )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table SMALLINT_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into SMALLINT_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{-9999999.99, -999999999999999.9999, -999999999999999.99999999},
			[]interface{}{-21474836, -2147483648, -2147483648},
			[]interface{}{-1234567, -123456792, -123456792},
			[]interface{}{-1.98, -1.9873, -1.98730468},
			[]interface{}{-1, -1, -1},
			[]interface{}{-0.76, -0.7617, -0.76171875},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from SMALLINT_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(-21474836), int64(-2147483648), int64(-2147483648)},
					[]interface{}{int64(-10000000), int64(-1000000000000000), int64(-1000000000000000)},
					[]interface{}{int64(-1234567), int64(-123456792), int64(-123456792)},
					[]interface{}{int64(-2), int64(-2), int64(-2)},
					[]interface{}{int64(-1), int64(-1), int64(-1)},
					[]interface{}{int64(-1), int64(-1), int64(-1)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from SMALLINT_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{-21474836},
			[]interface{}{-10000000},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from SMALLINT_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(-1234567), int64(-123456792), int64(-123456792)},
					[]interface{}{int64(-2), int64(-2), int64(-2)},
					[]interface{}{int64(-1), int64(-1), int64(-1)},
					[]interface{}{int64(-1), int64(-1), int64(-1)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// INT positive
	err = testExec(t, "truncate table SMALLINT_"+TestTimeString, nil)
	if err != nil {
		t.Error("truncate error:", err)
	}

	err = testExecRows(t, "insert into SMALLINT_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{0, 0, 0},
			[]interface{}{0.76, 0.7617, 0.76171875},
			[]interface{}{1, 1, 1},
			[]interface{}{1.98, 1.9873, 1.98730468},
			[]interface{}{12345679, 123456792, 123456792},
			[]interface{}{21474836, 2147483647, 2147483647},
			[]interface{}{9999999.99, 999999999999999.9999, 999999999999999.99999999},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from SMALLINT_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(0), int64(0), int64(0)},
					[]interface{}{int64(1), int64(1), int64(1)},
					[]interface{}{int64(1), int64(1), int64(1)},
					[]interface{}{int64(2), int64(2), int64(2)},
					[]interface{}{int64(10000000), int64(1000000000000000), int64(1000000000000000)},
					[]interface{}{int64(12345679), int64(123456792), int64(123456792)},
					[]interface{}{int64(21474836), int64(2147483647), int64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from SMALLINT_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{10000000},
			[]interface{}{12345679},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from SMALLINT_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(0), int64(0), int64(0)},
					[]interface{}{int64(1), int64(1), int64(1)},
					[]interface{}{int64(1), int64(1), int64(1)},
					[]interface{}{int64(2), int64(2), int64(2)},
					[]interface{}{int64(21474836), int64(2147483647), int64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// REAL negative
	err = testExec(t, "create table REAL_"+TestTimeString+
		" ( A REAL, B REAL, C REAL )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table REAL_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into REAL_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{-9999999.99, -999999999999999.9999, -9999999999999999999999999.99999999},
			[]interface{}{-21474836, -2147483648, -2147483648},
			[]interface{}{-1234567, -123456792, -123456792},
			[]interface{}{-1.98, -1.9873, -1.98730468},
			[]interface{}{-1, -1, -1},
			[]interface{}{-0.76, -0.7617, -0.76171875},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from REAL_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(-21474836), float64(-2147483648), float64(-2147483648)},
					[]interface{}{float64(-9999999.99), float64(-999999999999999.9999), float64(-9999999999999999999999999.99999999)},
					[]interface{}{float64(-1234567), float64(-123456792), float64(-123456792)},
					[]interface{}{float64(-1.98), float64(-1.9873), float64(-1.98730468)},
					[]interface{}{float64(-1), float64(-1), float64(-1)},
					[]interface{}{float64(-0.76), float64(-0.7617), float64(-0.76171875)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from REAL_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{-21474836},
			[]interface{}{-9999999.99},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from REAL_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(-1234567), float64(-123456792), float64(-123456792)},
					[]interface{}{float64(-1.98), float64(-1.9873), float64(-1.98730468)},
					[]interface{}{float64(-1), float64(-1), float64(-1)},
					[]interface{}{float64(-0.76), float64(-0.7617), float64(-0.76171875)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// REAL positive
	err = testExec(t, "truncate table REAL_"+TestTimeString, nil)
	if err != nil {
		t.Error("truncate error:", err)
	}

	err = testExecRows(t, "insert into REAL_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{0, 0, 0},
			[]interface{}{0.76, 0.7617, 0.76171875},
			[]interface{}{1, 1, 1},
			[]interface{}{1.98, 1.9873, 1.98730468},
			[]interface{}{12345679, 123456792, 123456792},
			[]interface{}{21474836, 2147483647, 2147483647},
			[]interface{}{9999999.99, 999999999999999.9999, 99999999999999999999999999.99999999},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from REAL_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(0), float64(0), float64(0)},
					[]interface{}{float64(0.76), float64(0.7617), float64(0.76171875)},
					[]interface{}{float64(1), float64(1), float64(1)},
					[]interface{}{float64(1.98), float64(1.9873), float64(1.98730468)},
					[]interface{}{float64(9999999.99), float64(999999999999999.9999), float64(99999999999999999999999999.99999999)},
					[]interface{}{float64(12345679), float64(123456792), float64(123456792)},
					[]interface{}{float64(21474836), float64(2147483647), float64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from REAL_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{0},
			[]interface{}{1},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from REAL_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(0.76), float64(0.7617), float64(0.76171875)},
					[]interface{}{float64(1.98), float64(1.9873), float64(1.98730468)},
					[]interface{}{float64(9999999.99), float64(999999999999999.9999), float64(99999999999999999999999999.99999999)},
					[]interface{}{float64(12345679), float64(123456792), float64(123456792)},
					[]interface{}{float64(21474836), float64(2147483647), float64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// BINARY_FLOAT negative
	err = testExec(t, "create table BINARY_FLOAT_"+TestTimeString+
		" ( A BINARY_FLOAT, B BINARY_FLOAT, C BINARY_FLOAT )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table BINARY_FLOAT_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into BINARY_FLOAT_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{float64(-288230381928101358902502915674136903680), float64(-288230381928101358902502915674136903680), float64(-288230381928101358902502915674136903680)},
			[]interface{}{-2147483648, -2147483648, -2147483648},
			[]interface{}{-123456792, -123456792, -123456792},
			[]interface{}{-1.99999988079071044921875, -1.99999988079071044921875, -1.99999988079071044921875},
			[]interface{}{-1, -1, -1},
			[]interface{}{-0.00415134616196155548095703125, -0.00415134616196155548095703125, -0.00415134616196155548095703125},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from BINARY_FLOAT_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(-288230381928101358902502915674136903680), float64(-288230381928101358902502915674136903680), float64(-288230381928101358902502915674136903680)},
					[]interface{}{float64(-2147483648), float64(-2147483648), float64(-2147483648)},
					[]interface{}{float64(-123456792), float64(-123456792), float64(-123456792)},
					[]interface{}{float64(-1.99999988079071044921875), float64(-1.99999988079071044921875), float64(-1.99999988079071044921875)},
					[]interface{}{float64(-1), float64(-1), float64(-1)},
					[]interface{}{float64(-0.00415134616196155548095703125), float64(-0.00415134616196155548095703125), float64(-0.00415134616196155548095703125)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from BINARY_FLOAT_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{-2147483648},
			[]interface{}{float64(-288230381928101358902502915674136903680)},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from BINARY_FLOAT_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(-123456792), float64(-123456792), float64(-123456792)},
					[]interface{}{float64(-1.99999988079071044921875), float64(-1.99999988079071044921875), float64(-1.99999988079071044921875)},
					[]interface{}{float64(-1), float64(-1), float64(-1)},
					[]interface{}{float64(-0.00415134616196155548095703125), float64(-0.00415134616196155548095703125), float64(-0.00415134616196155548095703125)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// BINARY_FLOAT positive
	err = testExec(t, "truncate table BINARY_FLOAT_"+TestTimeString, nil)
	if err != nil {
		t.Error("truncate error:", err)
	}

	err = testExecRows(t, "insert into BINARY_FLOAT_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{0, 0, 0},
			[]interface{}{0.00415134616196155548095703125, 0.00415134616196155548095703125, 0.00415134616196155548095703125},
			[]interface{}{1, 1, 1},
			[]interface{}{1.99999988079071044921875, 1.99999988079071044921875, 1.99999988079071044921875},
			[]interface{}{123456792, 123456792, 123456792},
			[]interface{}{2147483648, 2147483648, 2147483648},
			[]interface{}{float64(288230381928101358902502915674136903680), float64(288230381928101358902502915674136903680), float64(288230381928101358902502915674136903680)},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from BINARY_FLOAT_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(0), float64(0), float64(0)},
					[]interface{}{float64(0.00415134616196155548095703125), float64(0.00415134616196155548095703125), float64(0.00415134616196155548095703125)},
					[]interface{}{float64(1), float64(1), float64(1)},
					[]interface{}{float64(1.99999988079071044921875), float64(1.99999988079071044921875), float64(1.99999988079071044921875)},
					[]interface{}{float64(123456792), float64(123456792), float64(123456792)},
					[]interface{}{float64(2147483648), float64(2147483648), float64(2147483648)},
					[]interface{}{float64(288230381928101358902502915674136903680), float64(288230381928101358902502915674136903680), float64(288230381928101358902502915674136903680)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from BINARY_FLOAT_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{0},
			[]interface{}{1},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from BINARY_FLOAT_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(0.00415134616196155548095703125), float64(0.00415134616196155548095703125), float64(0.00415134616196155548095703125)},
					[]interface{}{float64(1.99999988079071044921875), float64(1.99999988079071044921875), float64(1.99999988079071044921875)},
					[]interface{}{float64(123456792), float64(123456792), float64(123456792)},
					[]interface{}{float64(2147483648), float64(2147483648), float64(2147483648)},
					[]interface{}{float64(288230381928101358902502915674136903680), float64(288230381928101358902502915674136903680), float64(288230381928101358902502915674136903680)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// BINARY_DOUBLE negative
	err = testExec(t, "create table BINARY_DOUBLE_"+TestTimeString+
		" ( A BINARY_DOUBLE, B BINARY_DOUBLE, C BINARY_DOUBLE )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table BINARY_DOUBLE_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into BINARY_DOUBLE_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{-9999999.99, -999999999999999.9999, -9999999999999999999999999.99999999},
			[]interface{}{-21474836, -2147483648, -2147483648},
			[]interface{}{-1234567, -123456792, -123456792},
			[]interface{}{-1.98, -1.9873, -1.98730468},
			[]interface{}{-1, -1, -1},
			[]interface{}{-0.76, -0.7617, -0.76171875},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from BINARY_DOUBLE_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(-21474836), float64(-2147483648), float64(-2147483648)},
					[]interface{}{float64(-9999999.99), float64(-999999999999999.9999), float64(-9999999999999999999999999.99999999)},
					[]interface{}{float64(-1234567), float64(-123456792), float64(-123456792)},
					[]interface{}{float64(-1.98), float64(-1.9873), float64(-1.98730468)},
					[]interface{}{float64(-1), float64(-1), float64(-1)},
					[]interface{}{float64(-0.76), float64(-0.7617), float64(-0.76171875)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from BINARY_DOUBLE_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{-21474836},
			[]interface{}{-9999999.99},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from BINARY_DOUBLE_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(-1234567), float64(-123456792), float64(-123456792)},
					[]interface{}{float64(-1.98), float64(-1.9873), float64(-1.98730468)},
					[]interface{}{float64(-1), float64(-1), float64(-1)},
					[]interface{}{float64(-0.76), float64(-0.7617), float64(-0.76171875)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// BINARY_DOUBLE positive
	err = testExec(t, "truncate table BINARY_DOUBLE_"+TestTimeString, nil)
	if err != nil {
		t.Error("truncate error:", err)
	}

	err = testExecRows(t, "insert into BINARY_DOUBLE_"+TestTimeString+" ( A, B, C ) values (:1, :2, :3)",
		[][]interface{}{
			[]interface{}{0, 0, 0},
			[]interface{}{0.76, 0.7617, 0.76171875},
			[]interface{}{1, 1, 1},
			[]interface{}{1.98, 1.9873, 1.98730468},
			[]interface{}{12345679, 123456792, 123456792},
			[]interface{}{21474836, 2147483647, 2147483647},
			[]interface{}{9999999.99, 999999999999999.9999, 99999999999999999999999999.99999999},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from BINARY_DOUBLE_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(0), float64(0), float64(0)},
					[]interface{}{float64(0.76), float64(0.7617), float64(0.76171875)},
					[]interface{}{float64(1), float64(1), float64(1)},
					[]interface{}{float64(1.98), float64(1.9873), float64(1.98730468)},
					[]interface{}{float64(9999999.99), float64(999999999999999.9999), float64(99999999999999999999999999.99999999)},
					[]interface{}{float64(12345679), float64(123456792), float64(123456792)},
					[]interface{}{float64(21474836), float64(2147483647), float64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from BINARY_DOUBLE_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{0},
			[]interface{}{1},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from BINARY_DOUBLE_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{float64(0.76), float64(0.7617), float64(0.76171875)},
					[]interface{}{float64(1.98), float64(1.9873), float64(1.98730468)},
					[]interface{}{float64(9999999.99), float64(999999999999999.9999), float64(99999999999999999999999999.99999999)},
					[]interface{}{float64(12345679), float64(123456792), float64(123456792)},
					[]interface{}{float64(21474836), float64(2147483647), float64(2147483647)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)
}

// TestDestructiveTime checks insert, select, update, and delete of time types
func TestDestructiveTime(t *testing.T) {
	if TestDisableDatabase || TestDisableDestructive {
		t.SkipNow()
	}

	// https://ss64.com/ora/syntax-datatypes.html

	// INTERVAL YEAR TO MONTH
	err := testExec(t, "create table INTERVALYTM_"+TestTimeString+
		" ( A int, B INTERVAL YEAR TO MONTH, C INTERVAL YEAR TO MONTH )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table INTERVALYTM_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into INTERVALYTM_"+TestTimeString+" ( A, B, C ) values (:1, NUMTOYMINTERVAL(:2, 'YEAR'), NUMTOYMINTERVAL(:3, 'MONTH'))",
		[][]interface{}{
			[]interface{}{1, -2, -2},
			[]interface{}{2, -1, -1},
			[]interface{}{3, 1, 1},
			[]interface{}{4, 2, 2},
			[]interface{}{5, 1.25, 2.1},
			[]interface{}{6, 1.5, 2.9},
			[]interface{}{7, 2.75, 3},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults := []testQueryResults{
		testQueryResults{
			query: "select A, B, C from INTERVALYTM_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(1), int64(-24), int64(-2)},
					[]interface{}{int64(2), int64(-12), int64(-1)},
					[]interface{}{int64(3), int64(12), int64(1)},
					[]interface{}{int64(4), int64(24), int64(2)},
					[]interface{}{int64(5), int64(15), int64(2)},
					[]interface{}{int64(6), int64(18), int64(3)},
					[]interface{}{int64(7), int64(33), int64(3)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from INTERVALYTM_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{5},
			[]interface{}{6},
			[]interface{}{7},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from INTERVALYTM_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(1), int64(-24), int64(-2)},
					[]interface{}{int64(2), int64(-12), int64(-1)},
					[]interface{}{int64(3), int64(12), int64(1)},
					[]interface{}{int64(4), int64(24), int64(2)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	// INTERVAL DAY TO SECOND
	err = testExec(t, "create table INTERVALDTS_"+TestTimeString+
		" ( A int, B INTERVAL DAY TO SECOND, C INTERVAL DAY TO SECOND )", nil)
	if err != nil {
		t.Fatal("create table error:", err)
	}

	defer func() {
		err = testExec(t, "drop table INTERVALDTS_"+TestTimeString, nil)
		if err != nil {
			t.Error("drop table error:", err)
		}
	}()

	err = testExecRows(t, "insert into INTERVALDTS_"+TestTimeString+" ( A, B, C ) values (:1, NUMTODSINTERVAL(:2, 'DAY'), NUMTODSINTERVAL(:3, 'HOUR'))",
		[][]interface{}{
			[]interface{}{1, -2, -2},
			[]interface{}{2, -1, -1},
			[]interface{}{3, 1, 1},
			[]interface{}{4, 2, 2},
			[]interface{}{5, 1.25, 1.25},
			[]interface{}{6, 1.5, 1.5},
			[]interface{}{7, 2.75, 2.75},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from INTERVALDTS_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(1), int64(-172800000000000), int64(-7200000000000)},
					[]interface{}{int64(2), int64(-86400000000000), int64(-3600000000000)},
					[]interface{}{int64(3), int64(86400000000000), int64(3600000000000)},
					[]interface{}{int64(4), int64(172800000000000), int64(7200000000000)},
					[]interface{}{int64(5), int64(108000000000000), int64(4500000000000)},
					[]interface{}{int64(6), int64(129600000000000), int64(5400000000000)},
					[]interface{}{int64(7), int64(237600000000000), int64(9900000000000)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from INTERVALDTS_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{5},
			[]interface{}{6},
			[]interface{}{7},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from INTERVALDTS_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(1), int64(-172800000000000), int64(-7200000000000)},
					[]interface{}{int64(2), int64(-86400000000000), int64(-3600000000000)},
					[]interface{}{int64(3), int64(86400000000000), int64(3600000000000)},
					[]interface{}{int64(4), int64(172800000000000), int64(7200000000000)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExec(t, "truncate table INTERVALDTS_"+TestTimeString, nil)
	if err != nil {
		t.Error("truncate error:", err)
	}

	err = testExecRows(t, "insert into INTERVALDTS_"+TestTimeString+" ( A, B, C ) values (:1, NUMTODSINTERVAL(:2, 'MINUTE'), NUMTODSINTERVAL(:3, 'SECOND'))",
		[][]interface{}{
			[]interface{}{1, -2, -2},
			[]interface{}{2, -1, -1},
			[]interface{}{3, 1, 1},
			[]interface{}{4, 2, 2},
			[]interface{}{5, 1.25, 1.25},
			[]interface{}{6, 1.5, 1.5},
			[]interface{}{7, 2.75, 2.75},
		})
	if err != nil {
		t.Error("insert error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from INTERVALDTS_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(1), int64(-120000000000), int64(-2000000000)},
					[]interface{}{int64(2), int64(-60000000000), int64(-1000000000)},
					[]interface{}{int64(3), int64(60000000000), int64(1000000000)},
					[]interface{}{int64(4), int64(120000000000), int64(2000000000)},
					[]interface{}{int64(5), int64(75000000000), int64(1250000000)},
					[]interface{}{int64(6), int64(90000000000), int64(1500000000)},
					[]interface{}{int64(7), int64(165000000000), int64(2750000000)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

	err = testExecRows(t, "delete from INTERVALDTS_"+TestTimeString+" where A = :1",
		[][]interface{}{
			[]interface{}{5},
			[]interface{}{6},
			[]interface{}{7},
		})
	if err != nil {
		t.Error("delete error:", err)
	}

	queryResults = []testQueryResults{
		testQueryResults{
			query: "select A, B, C from INTERVALDTS_" + TestTimeString + " order by A",
			args:  [][]interface{}{[]interface{}{}},
			results: [][][]interface{}{
				[][]interface{}{
					[]interface{}{int64(1), int64(-120000000000), int64(-2000000000)},
					[]interface{}{int64(2), int64(-60000000000), int64(-1000000000)},
					[]interface{}{int64(3), int64(60000000000), int64(1000000000)},
					[]interface{}{int64(4), int64(120000000000), int64(2000000000)},
				},
			},
		},
	}
	testRunQueryResults(t, queryResults)

}
