import wailsPlatform from "./wailsPlatform.js";
import webPlatform from "./webPlatform.js";

function hasWailsBindings() {
  return Boolean(window?.go?.main?.App);
}

const platform = hasWailsBindings() ? wailsPlatform : webPlatform;

export default platform;
