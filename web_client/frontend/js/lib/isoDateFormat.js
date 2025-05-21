export default function isoDateFormat(isoDate) {
  const date = new Date(isoDate);

  const year = date.getFullYear();
  const month = (date.getMonth() + 1).toString().padStart(2, "0"); // +1 because months are 0-indexed
  const day = date.getDate().toString().padStart(2, "0");

  const hours = date.getHours();
  const minutes = date.getMinutes();

  const formattedHours = hours % 12 || 12;
  const ampm = hours < 12 ? "am" : "pm";
  const formattedMinutes = minutes.toString().padStart(2, "0");

  return `${month}-${day}-${year} ${formattedHours}:${formattedMinutes}${ampm}`;
}
