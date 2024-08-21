package lru

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tinyverse-web3/neutrino/cache"
)

// sizeable is a simple struct that represents an element of arbitrary size
// which holds a simple integer.
type sizeable struct {
	value int
	size  uint64
}

// Size implements the CacheEntry interface on sizeable struct.
func (s *sizeable) Size() (uint64, error) {
	return s.size, nil
}

// getSizeableValue is a helper method used for converting the cache.Value
// interface to sizeable struct and extracting the value from it.
func getSizeableValue(generic cache.Value, _ error) int {
	return generic.(*sizeable).value
}

// TestEmptyCacheSizeZero will check that an empty cache has a size of 0.
func TestEmptyCacheSizeZero(t *testing.T) {
	t.Parallel()
	c := NewCache[int, *sizeable](10)
	require.Equal(t, 0, c.Len())
}

// TestCacheNeverExceedsSize inserts many filters into the cache and verifies
// at each step that the cache never exceeds it's initial size.
func TestCacheNeverExceedsSize(t *testing.T) {
	t.Parallel()
	c := NewCache[int, *sizeable](2)
	c.Put(1, &sizeable{value: 1, size: 1})
	c.Put(2, &sizeable{value: 2, size: 1})
	require.Equal(t, 2, c.Len())
	for i := 0; i < 10; i++ {
		c.Put(i, &sizeable{value: i, size: 1})
		require.Equal(t, 2, c.Len())
	}
}

// TestCacheAlwaysHasLastAccessedItems will check that the last items that
// were put in the cache are always available, it will also check the eviction
// behavior when items put in the cache exceeds cache capacity.
func TestCacheAlwaysHasLastAccessedItems(t *testing.T) {
	t.Parallel()
	c := NewCache[int, *sizeable](2)
	c.Put(1, &sizeable{value: 1, size: 1})
	c.Put(2, &sizeable{value: 2, size: 1})
	two := getSizeableValue(c.Get(2))
	one := getSizeableValue(c.Get(1))
	require.Equal(t, 2, two)
	require.Equal(t, 1, one)

	c = NewCache[int, *sizeable](2)
	c.Put(1, &sizeable{value: 1, size: 1})
	c.Put(2, &sizeable{value: 2, size: 1})
	c.Put(3, &sizeable{value: 3, size: 1})
	_, err := c.Get(1)
	two = getSizeableValue(c.Get(2))
	three := getSizeableValue(c.Get(3))
	require.ErrorIs(t, err, cache.ErrElementNotFound)
	require.Equal(t, two, 2)
	require.Equal(t, three, 3)

	c = NewCache[int, *sizeable](2)
	c.Put(1, &sizeable{value: 1, size: 1})
	c.Put(2, &sizeable{value: 2, size: 1})
	c.Get(1)
	c.Put(3, &sizeable{value: 3, size: 1})
	one = getSizeableValue(c.Get(1))
	_, err = c.Get(2)
	three = getSizeableValue(c.Get(3))
	require.Equal(t, one, 1)
	require.ErrorIs(t, err, cache.ErrElementNotFound)
	require.Equal(t, three, 3)
}

// TestElementSizeCapacityEvictsEverything tests that Cache evicts everything
// from cache when an element with size=capacity is inserted.
func TestElementSizeCapacityEvictsEverything(t *testing.T) {
	t.Parallel()
	c := NewCache[int, *sizeable](3)

	c.Put(1, &sizeable{value: 1, size: 1})
	c.Put(2, &sizeable{value: 2, size: 1})
	c.Put(3, &sizeable{value: 3, size: 1})

	// Insert element with size=capacity of cache, should evict everything.
	c.Put(4, &sizeable{value: 4, size: 3})
	require.Equal(t, c.Len(), 1)
	require.Equal(t, c.cache.Len(), 1)
	four := getSizeableValue(c.Get(4))
	require.Equal(t, four, 4)

	c = NewCache[int, *sizeable](6)
	c.Put(1, &sizeable{value: 1, size: 1})
	c.Put(2, &sizeable{value: 2, size: 2})
	c.Put(3, &sizeable{value: 3, size: 3})
	require.Equal(t, c.size, uint64(6))

	// Insert element with size=capacity of cache.
	c.Put(4, &sizeable{value: 4, size: 6})
	require.Equal(t, c.Len(), 1)
	require.Equal(t, c.cache.Len(), 1)
	four = getSizeableValue(c.Get(4))
	require.Equal(t, four, 4)
}

// TestCacheFailsInsertionSizeBiggerCapacity tests that the cache fails the
// put operation when the element's size is bigger than it's capacity.
func TestCacheFailsInsertionSizeBiggerCapacity(t *testing.T) {
	t.Parallel()
	c := NewCache[int, *sizeable](2)

	_, err := c.Put(1, &sizeable{value: 1, size: 3})
	if err == nil {
		t.Fatal("shouldn't be able to put elements larger than cache")
	}
	require.Equal(t, c.Len(), 0)
}

// TestManySmallElementCanInsertAfterBigEviction tests that when a big element
// is evicted from the Cache, multiple smaller ones can be inserted without an
// eviction taking place.
func TestManySmallElementCanInsertAfterBigEviction(t *testing.T) {
	t.Parallel()
	c := NewCache[int, *sizeable](3)

	_, err := c.Put(1, &sizeable{value: 1, size: 3})
	if err != nil {
		t.Fatal("couldn't insert element")
	}

	require.Equal(t, c.Len(), 1)

	c.Put(2, &sizeable{value: 2, size: 1})
	two := getSizeableValue(c.Get(2))
	_, err = c.Get(1)
	require.Equal(t, c.Len(), 1)
	require.Equal(t, two, 2)
	require.ErrorIs(t, err, cache.ErrElementNotFound)

	c.Put(3, &sizeable{value: 3, size: 1})
	require.Equal(t, c.Len(), 2)

	c.Put(4, &sizeable{value: 4, size: 1})
	require.Equal(t, c.Len(), 3)

	two = getSizeableValue(c.Get(2))
	three := getSizeableValue(c.Get(3))
	four := getSizeableValue(c.Get(4))
	require.Equal(t, two, 2)
	require.Equal(t, three, 3)
	require.Equal(t, four, 4)
}

// TestReplacingElementValueSmallerSize tests that if an existing element is
// replaced with a value of size smaller, that the size shrinks and we can
// insert without an eviction taking place.
func TestReplacingElementValueSmallerSize(t *testing.T) {
	t.Parallel()
	c := NewCache[int, *sizeable](2)

	c.Put(1, &sizeable{value: 1, size: 2})

	c.Put(1, &sizeable{value: 1, size: 1})
	c.Put(2, &sizeable{value: 2, size: 1})
	one := getSizeableValue(c.Get(1))
	two := getSizeableValue(c.Get(2))
	require.Equal(t, one, 1)
	require.Equal(t, two, 2)
	require.Equal(t, c.Len(), 2)
}

// TestReplacingElementValueBiggerSize tests that if an existing element is
// replaced with a value of size bigger, that it evicts accordingly.
func TestReplacingElementValueBiggerSize(t *testing.T) {
	t.Parallel()
	c := NewCache[int, *sizeable](2)

	c.Put(1, &sizeable{value: 1, size: 1})
	c.Put(2, &sizeable{value: 2, size: 1})

	c.Put(1, &sizeable{value: 3, size: 2})
	require.Equal(t, c.Len(), 1)
	one := getSizeableValue(c.Get(1))
	require.Equal(t, one, 3)
}

// TestConcurrencySimple is a very simple test that checks concurrent access to
// the lru cache. When running the test, "-race" option should be passed to
// "go test" command.
func TestConcurrencySimple(t *testing.T) {
	t.Parallel()
	c := NewCache[int, *sizeable](5)
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := c.Put(i, &sizeable{value: i, size: 1})
			if err != nil {
				t.Error(err)
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := c.Get(i)
			if err != nil && err != cache.ErrElementNotFound {
				t.Error(err)
			}
		}(i)
	}

	wg.Wait()
}

// TestConcurrencySmallCache is a test that checks concurrent access to the
// lru cache when the cache is smaller than the number of elements we want to
// put and retrieve. When running the test, "-race" option should be passed to
// "go test" command.
func TestConcurrencySmallCache(t *testing.T) {
	t.Parallel()
	c := NewCache[int, *sizeable](5)
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := c.Put(i, &sizeable{value: i, size: 1})
			if err != nil {
				t.Error(err)
			}
		}(i)
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := c.Get(i)
			if err != nil && err != cache.ErrElementNotFound {
				t.Error(err)
			}
		}(i)
	}

	wg.Wait()
}

// TestConcurrencyBigCache is a test that checks concurrent access to the
// lru cache when the cache is bigger than the number of elements we want to
// put and retrieve. When running the test, "-race" option should be passed to
// "go test" command.
func TestConcurrencyBigCache(t *testing.T) {
	t.Parallel()
	c := NewCache[int, *sizeable](100)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := c.Put(i, &sizeable{value: i, size: 1})
			if err != nil {
				t.Error(err)
			}
		}(i)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := c.Get(i)
			if err != nil && err != cache.ErrElementNotFound {
				t.Error(err)
			}
		}(i)
	}

	wg.Wait()
}

// TestLoadAndDelete checks the `LoadAndDelete` method.
func TestLoadAndDelete(t *testing.T) {
	t.Parallel()

	c := NewCache[int, *sizeable](3)

	// Create a test item.
	item1 := &sizeable{value: 1, size: 1}

	// Put the item.
	_, err := c.Put(0, item1)
	require.NoError(t, err)

	// Load the item and check it's returned as expected.
	loadedItem, loaded := c.LoadAndDelete(0)
	require.True(t, loaded)
	require.Equal(t, item1, loadedItem)

	// Now check that the item has been deleted.
	_, err = c.Get(0)
	require.ErrorIs(t, err, cache.ErrElementNotFound)

	// Load the item again should give us a nil value and false.
	loadedItem, loaded = c.LoadAndDelete(0)
	require.False(t, loaded)
	require.Nil(t, loadedItem)

	// The length should be 0.
	require.Zero(t, c.Len())
	require.Zero(t, c.size)
}

// TestRangeIteration checks that the `Range` method works as expected.
func TestRangeIteration(t *testing.T) {
	t.Parallel()

	c := NewCache[int, *sizeable](100)

	// Create test items.
	const numItems = 10
	for i := 0; i < numItems; i++ {
		_, err := c.Put(i, &sizeable{value: i, size: 1})
		require.NoError(t, err)
	}

	// Create a dummy visitor that just counts the number of items visited.
	visited := 0
	testVisitor := func(key int, value *sizeable) bool {
		visited++
		return true
	}

	// Call the method.
	c.Range(testVisitor)

	// Check the number of items visited.
	require.Equal(t, numItems, visited)
}

// TestRangeAbort checks that the `Range` will abort when the visitor returns
// false.
func TestRangeAbort(t *testing.T) {
	t.Parallel()

	c := NewCache[int, *sizeable](100)

	// Create test items.
	const numItems = 10
	for i := 0; i < numItems; i++ {
		_, err := c.Put(i, &sizeable{value: i, size: 1})
		require.NoError(t, err)
	}

	// Create a visitor that counts the number of items visited and returns
	// false when visited 5 times.
	visited := 0
	testVisitor := func(key int, value *sizeable) bool {
		visited++
		if visited >= numItems/2 {
			return false
		}
		return true
	}

	// Call the method.
	c.Range(testVisitor)

	// Check the number of items visited.
	require.Equal(t, numItems/2, visited)
}

// TestRangeFILO checks that the `RangeFILO` method works as expected.
func TestRangeFILO(t *testing.T) {
	t.Parallel()

	c := NewCache[int, *sizeable](100)

	// Create test items.
	const numItems = 10
	for i := 0; i < numItems; i++ {
		_, err := c.Put(i, &sizeable{value: i, size: 1})
		require.NoError(t, err)
	}

	// Create a visitor that checks the items are visited in reverse order.
	visited := 0
	testVisitor := func(key int, value *sizeable) bool {
		visited++

		require.Equal(t, numItems-visited, key)
		return true
	}

	// Call the method.
	c.RangeFILO(testVisitor)

	// Check the number of items visited.
	require.Equal(t, numItems, visited)
}

// TestRangeFIFO checks that the `RangeFIFO` method works as expected.
func TestRangeFIFO(t *testing.T) {
	t.Parallel()

	c := NewCache[int, *sizeable](100)

	// Create test items.
	const numItems = 10
	for i := 0; i < numItems; i++ {
		_, err := c.Put(i, &sizeable{value: i, size: 1})
		require.NoError(t, err)
	}

	// Create a visitor that checks the items are visited in order.
	visited := 0
	testVisitor := func(key int, value *sizeable) bool {
		require.Equal(t, visited, key)
		visited++

		return true
	}

	// Call the method.
	c.RangeFIFO(testVisitor)

	// Check the number of items visited.
	require.Equal(t, numItems, visited)
}
