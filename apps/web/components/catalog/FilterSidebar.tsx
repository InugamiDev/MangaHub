export function FilterSidebar({
  genres = [],
  selectedGenre = "",
  selectedStatus = "",
  hiddenFields = {},
}: {
  genres?: string[];
  selectedGenre?: string;
  selectedStatus?: string;
  hiddenFields?: Record<string, string>;
}) {
  return (
    <form className="filter-sidebar">
      <h2>Filters</h2>
      {Object.entries(hiddenFields).map(([name, value]) => (
        value ? <input key={name} type="hidden" name={name} value={value} /> : null
      ))}
      <label>
        Genre
        <select name="genre" defaultValue={selectedGenre}>
          <option value="">All genres</option>
          {genres.map((genre) => (
            <option value={genre} key={genre}>{genre}</option>
          ))}
        </select>
      </label>
      <label>
        Status
        <select name="status" defaultValue={selectedStatus}>
          <option value="">Any status</option>
          <option>Ongoing</option>
          <option>Completed</option>
          <option>Hiatus</option>
          <option>Cancelled</option>
          <option>Not Yet Released</option>
        </select>
      </label>
      <button type="submit">Apply filters</button>
    </form>
  );
}
