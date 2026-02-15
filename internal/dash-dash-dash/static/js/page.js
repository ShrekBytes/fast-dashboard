import { setupPopovers } from './popover.js';
import { setupMasonries } from './masonry.js';
import { throttledDebounce, isElementVisible } from './utils.js';

const PAGE_CONTENT_FETCH_TIMEOUT_MS = 30000;
const PAGE_CONTENT_RETRY_DELAY_MS = 1500;
const PAGE_CONTENT_CACHE_KEY_PREFIX = 'dash-content-';
const PAGE_CONTENT_CACHE_TTL_MS = 2 * 60 * 1000; // 2 minutes

function getCachedContent(slug) {
    try {
        const key = PAGE_CONTENT_CACHE_KEY_PREFIX + (slug || '');
        const raw = localStorage.getItem(key);
        if (!raw) return null;
        const { html, fetchedAt } = JSON.parse(raw);
        if (typeof html !== 'string' || typeof fetchedAt !== 'number') return null;
        if (Date.now() - fetchedAt > PAGE_CONTENT_CACHE_TTL_MS) return null;
        return html;
    } catch (_) {
        return null;
    }
}

function setCachedContent(slug, html) {
    try {
        const key = PAGE_CONTENT_CACHE_KEY_PREFIX + (slug || '');
        localStorage.setItem(key, JSON.stringify({ html, fetchedAt: Date.now() }));
    } catch (_) {}
}

async function fetchPageContent(pageData, retryCount = 0) {
    const base = pageData.basePath || '';
    const url = `${base}/api/pages/${pageData.slug}/content/`;
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), PAGE_CONTENT_FETCH_TIMEOUT_MS);
    try {
        const response = await fetch(url, { signal: controller.signal });
        clearTimeout(timeoutId);
        const content = await response.text();
        if (!response.ok) {
            return { html: `<div class="widget-content padding-inline-widget" style="color: var(--color-negative);">Failed to load (${response.status}). <a href="javascript:location.reload()">Reload</a>.</div>`, ok: false };
        }
        return { html: content, ok: true };
    } catch (err) {
        clearTimeout(timeoutId);
        const isAbort = err.name === 'AbortError' || (err.message && /failed to fetch|load failed|aborted|cancelled/i.test(err.message));
        const message = isAbort ? 'Request was cancelled.' : (err.message || 'Network error.');
        const errorHtml = `<div class="widget-content padding-inline-widget" style="color: var(--color-negative);">${message} <a href="javascript:location.reload()">Reload</a>.</div>`;
        if (isAbort) {
            return { html: errorHtml, ok: false };
        }
        if (retryCount < 1) {
            await new Promise(r => setTimeout(r, PAGE_CONTENT_RETRY_DELAY_MS));
            return fetchPageContent(pageData, retryCount + 1);
        }
        return { html: errorHtml, ok: false };
    }
}

function setupCarousels() {
    const carouselElements = document.getElementsByClassName("carousel-container");

    if (carouselElements.length == 0) {
        return;
    }

    for (let i = 0; i < carouselElements.length; i++) {
        const carousel = carouselElements[i];
        carousel.classList.add("show-right-cutoff");
        const itemsContainer = carousel.getElementsByClassName("carousel-items-container")[0];

        const determineSideCutoffs = () => {
            if (itemsContainer.scrollLeft != 0) {
                carousel.classList.add("show-left-cutoff");
            } else {
                carousel.classList.remove("show-left-cutoff");
            }

            if (Math.ceil(itemsContainer.scrollLeft) + itemsContainer.clientWidth < itemsContainer.scrollWidth) {
                carousel.classList.add("show-right-cutoff");
            } else {
                carousel.classList.remove("show-right-cutoff");
            }
        }

        const determineSideCutoffsRateLimited = throttledDebounce(determineSideCutoffs, 20, 100);

        itemsContainer.addEventListener("scroll", determineSideCutoffsRateLimited);
        window.addEventListener("resize", determineSideCutoffsRateLimited);

        afterContentReady(determineSideCutoffs);
    }
}

const minuteInSeconds = 60;
const hourInSeconds = minuteInSeconds * 60;
const dayInSeconds = hourInSeconds * 24;
const monthInSeconds = dayInSeconds * 30.4;
const yearInSeconds = dayInSeconds * 365;

function timestampToRelativeTime(timestamp) {
    let delta = Math.round((Date.now() / 1000) - timestamp);
    let prefix = "";

    if (delta < 0) {
        delta = -delta;
        prefix = "in ";
    }

    if (delta < minuteInSeconds) {
        return prefix + "1m";
    }
    if (delta < hourInSeconds) {
        return prefix + Math.floor(delta / minuteInSeconds) + "m";
    }
    if (delta < dayInSeconds) {
        return prefix + Math.floor(delta / hourInSeconds) + "h";
    }
    if (delta < monthInSeconds) {
        return prefix + Math.floor(delta / dayInSeconds) + "d";
    }
    if (delta < yearInSeconds) {
        return prefix + Math.floor(delta / monthInSeconds) + "mo";
    }

    return prefix + Math.floor(delta / yearInSeconds) + "y";
}

function updateRelativeTimeForElements(elements)
{
    for (let i = 0; i < elements.length; i++)
    {
        const element = elements[i];
        const timestamp = element.dataset.dynamicRelativeTime;

        if (timestamp === undefined)
            continue

        element.textContent = timestampToRelativeTime(timestamp);
    }
}

function looksLikeUrl(input) {
    const trimmed = input.trim();
    if (trimmed.includes(" ")) return false;
    try {
        if (/^https?:\/\//i.test(trimmed)) {
            new URL(trimmed);
            return true;
        }
        if (/^[a-zA-Z0-9][a-zA-Z0-9.-]*\.[a-zA-Z]{2,}/.test(trimmed)) {
            new URL("https://" + trimmed);
            return true;
        }
    } catch (_) {}
    return false;
}

function setupSearchBoxes() {
    const searchWidgets = document.getElementsByClassName("search");

    if (searchWidgets.length == 0) {
        return;
    }

    for (let i = 0; i < searchWidgets.length; i++) {
        const widget = searchWidgets[i];
        const defaultSearchUrl = widget.dataset.defaultSearchUrl;
        const target = widget.dataset.target || "_blank";
        const newTab = widget.dataset.newTab === "true";
        const inputElement = widget.getElementsByClassName("search-input")[0];
        const bangElement = widget.getElementsByClassName("search-bang")[0];
        const bangs = widget.querySelectorAll(".search-bangs > input");
        const bangsMap = {};
        const kbdElement = widget.getElementsByTagName("kbd")[0];
        let currentBang = null;
        let lastQuery = "";

        for (let j = 0; j < bangs.length; j++) {
            const bang = bangs[j];
            bangsMap[bang.dataset.shortcut] = bang;
        }

        const handleKeyDown = (event) => {
            if (event.key == "Escape") {
                inputElement.blur();
                return;
            }

            if (event.key == "Enter") {
                const input = inputElement.value.trim();
                let query;
                let searchUrlTemplate;

                if (currentBang != null) {
                    query = input.slice(currentBang.dataset.shortcut.length + 1);
                    searchUrlTemplate = currentBang.dataset.url;
                } else {
                    if (input.length === 0) {
                        return;
                    }
                    if (looksLikeUrl(input)) {
                        let url = input;
                        if (!/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(url)) {
                            url = "https://" + url;
                        }
                        if (newTab && !event.ctrlKey || !newTab && event.ctrlKey) {
                            window.open(url, target).focus();
                        } else {
                            window.location.href = url;
                        }
                        inputElement.value = "";
                        return;
                    }
                    query = input;
                    searchUrlTemplate = defaultSearchUrl;
                }

                const url = searchUrlTemplate.replace("!QUERY!", encodeURIComponent(query));

                if (newTab && !event.ctrlKey || !newTab && event.ctrlKey) {
                    window.open(url, target).focus();
                } else {
                    window.location.href = url;
                }

                lastQuery = query;
                inputElement.value = "";

                return;
            }

            if (event.key == "ArrowUp" && lastQuery.length > 0) {
                inputElement.value = lastQuery;
                return;
            }
        };

        const changeCurrentBang = (bang) => {
            currentBang = bang;
            bangElement.textContent = bang != null ? bang.dataset.title : "";
        }

        const handleInput = (event) => {
            const value = event.target.value.trim();
            if (value in bangsMap) {
                changeCurrentBang(bangsMap[value]);
                return;
            }

            const words = value.split(" ");
            if (words.length >= 2 && words[0] in bangsMap) {
                changeCurrentBang(bangsMap[words[0]]);
                return;
            }

            changeCurrentBang(null);
        };

        inputElement.addEventListener("focus", () => {
            document.addEventListener("keydown", handleKeyDown);
            document.addEventListener("input", handleInput);
        });
        inputElement.addEventListener("blur", () => {
            document.removeEventListener("keydown", handleKeyDown);
            document.removeEventListener("input", handleInput);
        });

        document.addEventListener("keydown", (event) => {
            if (['INPUT', 'TEXTAREA'].includes(document.activeElement.tagName)) return;
            if (event.code != "KeyS") return;

            inputElement.focus();
            event.preventDefault();
        });

        kbdElement.addEventListener("mousedown", () => {
            requestAnimationFrame(() => inputElement.focus());
        });
    }
}

function setupDynamicRelativeTime() {
    const elements = document.querySelectorAll("[data-dynamic-relative-time]");
    const updateInterval = 60 * 1000;
    let lastUpdateTime = Date.now();

    updateRelativeTimeForElements(elements);

    const updateElementsAndTimestamp = () => {
        updateRelativeTimeForElements(elements);
        lastUpdateTime = Date.now();
    };

    const scheduleRepeatingUpdate = () => setInterval(updateElementsAndTimestamp, updateInterval);

    if (document.hidden === undefined) {
        scheduleRepeatingUpdate();
        return;
    }

    let timeout = scheduleRepeatingUpdate();

    document.addEventListener("visibilitychange", () => {
        if (document.hidden) {
            clearTimeout(timeout);
            return;
        }

        const delta = Date.now() - lastUpdateTime;

        if (delta >= updateInterval) {
            updateElementsAndTimestamp();
            timeout = scheduleRepeatingUpdate();
            return;
        }

        timeout = setTimeout(() => {
            updateElementsAndTimestamp();
            timeout = scheduleRepeatingUpdate();
        }, updateInterval - delta);
    });
}

function setupLazyImages() {
    const images = document.querySelectorAll("img[loading=lazy]");

    if (images.length == 0) {
        return;
    }

    function imageFinishedTransition(image) {
        image.classList.add("finished-transition");
    }

    afterContentReady(() => {
        setTimeout(() => {
            for (let i = 0; i < images.length; i++) {
                const image = images[i];

                if (image.complete) {
                    image.classList.add("cached");
                    setTimeout(() => imageFinishedTransition(image), 1);
                } else {
                    // TODO: also handle error event
                    image.addEventListener("load", () => {
                        image.classList.add("loaded");
                        setTimeout(() => imageFinishedTransition(image), 400);
                    });
                }
            }
        }, 1);
    });
}

function attachExpandToggleButton(collapsibleContainer) {
    const showMoreText = "Show more";
    const showLessText = "Show less";

    let expanded = false;
    const button = document.createElement("button");
    const icon = document.createElement("span");
    icon.classList.add("expand-toggle-button-icon");
    const textNode = document.createTextNode(showMoreText);
    button.classList.add("expand-toggle-button");
    button.append(textNode, icon);
    button.addEventListener("click", () => {
        expanded = !expanded;

        if (expanded) {
            collapsibleContainer.classList.add("container-expanded");
            button.classList.add("container-expanded");
            textNode.nodeValue = showLessText;
            return;
        }

        const topBefore = button.getClientRects()[0].top;

        collapsibleContainer.classList.remove("container-expanded");
        button.classList.remove("container-expanded");
        textNode.nodeValue = showMoreText;

        const topAfter = button.getClientRects()[0].top;

        if (topAfter > 0)
            return;

        window.scrollBy({
            top: topAfter - topBefore,
            behavior: "instant"
        });
    });

    collapsibleContainer.after(button);

    return button;
};


function setupCollapsibleLists() {
    const collapsibleLists = document.querySelectorAll(".list.collapsible-container");

    if (collapsibleLists.length == 0) {
        return;
    }

    for (let i = 0; i < collapsibleLists.length; i++) {
        const list = collapsibleLists[i];

        if (list.dataset.collapseAfter === undefined) {
            continue;
        }

        const collapseAfter = parseInt(list.dataset.collapseAfter);

        if (collapseAfter == -1) {
            continue;
        }

        if (list.children.length <= collapseAfter) {
            continue;
        }

        attachExpandToggleButton(list);

        for (let c = collapseAfter; c < list.children.length; c++) {
            const child = list.children[c];
            child.classList.add("collapsible-item");
            child.style.animationDelay = ((c - collapseAfter) * 20).toString() + "ms";
        }
    }
}

function setupCollapsibleGrids() {
    const collapsibleGridElements = document.querySelectorAll(".cards-grid.collapsible-container");

    if (collapsibleGridElements.length == 0) {
        return;
    }

    for (let i = 0; i < collapsibleGridElements.length; i++) {
        const gridElement = collapsibleGridElements[i];

        if (gridElement.dataset.collapseAfterRows === undefined) {
            continue;
        }

        const collapseAfterRows = parseInt(gridElement.dataset.collapseAfterRows);

        if (collapseAfterRows == -1) {
            continue;
        }

        const getCardsPerRow = () => {
            return parseInt(getComputedStyle(gridElement).getPropertyValue('--cards-per-row'));
        };

        const button = attachExpandToggleButton(gridElement);

        let cardsPerRow;

        const resolveCollapsibleItems = () => requestAnimationFrame(() => {
            const hideItemsAfterIndex = cardsPerRow * collapseAfterRows;

            if (hideItemsAfterIndex >= gridElement.children.length) {
                button.style.display = "none";
            } else {
                button.style.removeProperty("display");
            }

            let row = 0;

            for (let i = 0; i < gridElement.children.length; i++) {
                const child = gridElement.children[i];

                if (i >= hideItemsAfterIndex) {
                    child.classList.add("collapsible-item");
                    child.style.animationDelay = (row * 40).toString() + "ms";

                    if (i % cardsPerRow + 1 == cardsPerRow) {
                        row++;
                    }
                } else {
                    child.classList.remove("collapsible-item");
                    child.style.removeProperty("animation-delay");
                }
            }
        });

        const observer = new ResizeObserver(() => {
            if (!isElementVisible(gridElement)) {
                return;
            }

            const newCardsPerRow = getCardsPerRow();

            if (cardsPerRow == newCardsPerRow) {
                return;
            }

            cardsPerRow = newCardsPerRow;
            resolveCollapsibleItems();
        });

        afterContentReady(() => observer.observe(gridElement));
    }
}

const contentReadyCallbacks = [];

function afterContentReady(callback) {
    contentReadyCallbacks.push(callback);
}

const weekDayNames = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
const monthNames = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];

function makeSettableTimeElement(element, hourFormat) {
    const fragment = document.createDocumentFragment();
    const hour = document.createElement('span');
    const minute = document.createElement('span');
    const amPm = document.createElement('span');
    fragment.append(hour, document.createTextNode(':'), minute);

    if (hourFormat == '12h') {
        fragment.append(document.createTextNode(' '), amPm);
    }

    element.append(fragment);

    return (date) => {
        const hours = date.getHours();

        if (hourFormat == '12h') {
            amPm.textContent = hours < 12 ? 'AM' : 'PM';
            hour.textContent = hours % 12 || 12;
        } else {
            hour.textContent = hours < 10 ? '0' + hours : hours;
        }

        const minutes = date.getMinutes();
        minute.textContent = minutes < 10 ? '0' + minutes : minutes;
    };
};

function timeInZone(now, zone) {
    let timeInZone;

    try {
        timeInZone = new Date(now.toLocaleString('en-US', { timeZone: zone }));
    } catch (e) {
        // TODO: indicate to the user that this is an invalid timezone
        console.error(e);
        timeInZone = now
    }

    const diffInMinutes = Math.round((timeInZone.getTime() - now.getTime()) / 1000 / 60);

    return { time: timeInZone, diffInMinutes: diffInMinutes };
}

function zoneDiffText(diffInMinutes) {
    if (diffInMinutes == 0) {
        return "";
    }

    const sign = diffInMinutes < 0 ? "-" : "+";
    const signText = diffInMinutes < 0 ? "behind" : "ahead";

    diffInMinutes = Math.abs(diffInMinutes);

    const hours = Math.floor(diffInMinutes / 60);
    const minutes = diffInMinutes % 60;
    const hourSuffix = hours == 1 ? "" : "s";

    if (minutes == 0) {
        return { text: `${sign}${hours}h`, title: `${hours} hour${hourSuffix} ${signText}` };
    }

    if (hours == 0) {
        return { text: `${sign}${minutes}m`, title: `${minutes} minutes ${signText}` };
    }

    return { text: `${sign}${hours}h~`, title: `${hours} hour${hourSuffix} and ${minutes} minutes ${signText}` };
}

function setupClocks() {
    const clocks = document.getElementsByClassName('clock');

    if (clocks.length == 0) {
        return;
    }

    const updateCallbacks = [];

    for (var i = 0; i < clocks.length; i++) {
        const clock = clocks[i];
        const hourFormat = clock.dataset.hourFormat;
        const localTimeContainer = clock.querySelector('[data-local-time]');
        const localDateElement = localTimeContainer.querySelector('[data-date]');
        const localWeekdayElement = localTimeContainer.querySelector('[data-weekday]');
        const localYearElement = localTimeContainer.querySelector('[data-year]');
        const timeZoneContainers = clock.querySelectorAll('[data-time-in-zone]');

        const setLocalTime = makeSettableTimeElement(
            localTimeContainer.querySelector('[data-time]'),
            hourFormat
        );

        updateCallbacks.push((now) => {
            setLocalTime(now);
            localDateElement.textContent = now.getDate() + ' ' + monthNames[now.getMonth()];
            localWeekdayElement.textContent = weekDayNames[now.getDay()];
            localYearElement.textContent = now.getFullYear();
        });

        for (var z = 0; z < timeZoneContainers.length; z++) {
            const timeZoneContainer = timeZoneContainers[z];
            const diffElement = timeZoneContainer.querySelector('[data-time-diff]');

            const setZoneTime = makeSettableTimeElement(
                timeZoneContainer.querySelector('[data-time]'),
                hourFormat
            );

            updateCallbacks.push((now) => {
                const { time, diffInMinutes } = timeInZone(now, timeZoneContainer.dataset.timeInZone);
                setZoneTime(time);
                const { text, title } = zoneDiffText(diffInMinutes);
                diffElement.textContent = text;
                diffElement.title = title;
            });
        }
    }

    const updateClocks = () => {
        const now = new Date();

        for (var i = 0; i < updateCallbacks.length; i++)
            updateCallbacks[i](now);

        setTimeout(updateClocks, (60 - now.getSeconds()) * 1000);
    };

    updateClocks();
}

async function setupCalendars() {
    const elems = document.getElementsByClassName("calendar");
    if (elems.length == 0) return;

    // TODO: implement prefetching, currently loads as a nasty waterfall of requests
    const calendar = await import ('./calendar.js');

    for (let i = 0; i < elems.length; i++)
        calendar.default(elems[i]);
}

async function setupTodos() {
    const elems = Array.from(document.getElementsByClassName("todo"));
    if (elems.length == 0) return;

    const todo = await import ('./todo.js');

    for (let i = 0; i < elems.length; i++){
        todo.default(elems[i]);
    }
}

function setupTruncatedElementTitles() {
    const elements = document.querySelectorAll(".text-truncate, .single-line-titles .title, .text-truncate-2-lines, .text-truncate-3-lines");

    if (elements.length == 0) {
        return;
    }

    for (let i = 0; i < elements.length; i++) {
        const element = elements[i];
        if (element.getAttribute("title") === null)
            element.title = element.innerText.trim().replace(/\s+/g, " ");
    }
}

async function applyContentAndSetup(pageElement, pageContentElement, html) {
    pageContentElement.innerHTML = html;
    contentReadyCallbacks.length = 0; // Only run callbacks registered during this apply (avoids stale refs when re-applying after background refetch).

    try {
        setupPopovers();
        setupClocks();
        await setupCalendars();
        await setupTodos();
        setupCarousels();
        setupSearchBoxes();
        setupCollapsibleLists();
        setupCollapsibleGrids();
        setupMasonries();
        setupDynamicRelativeTime();
        setupLazyImages();
    } finally {
        pageElement.classList.add("content-ready");
        pageElement.setAttribute("aria-busy", "false");

        for (let i = 0; i < contentReadyCallbacks.length; i++) {
            contentReadyCallbacks[i]();
        }

        setTimeout(() => {
            setupTruncatedElementTitles();
        }, 50);

        setTimeout(() => {
            document.body.classList.add("page-columns-transitioned");
        }, 300);
    }
}

async function setupPage() {
    const pageElement = document.getElementById("page");
    const pageContentElement = document.getElementById("page-content");

    const cached = getCachedContent(pageData.slug);
    if (cached !== null) {
        await applyContentAndSetup(pageElement, pageContentElement, cached);
        // Re-fetch in background and replace when ready (stale-while-revalidate).
        const result = await fetchPageContent(pageData);
        if (result.ok && result.html !== cached) {
            setCachedContent(pageData.slug, result.html);
            await applyContentAndSetup(pageElement, pageContentElement, result.html);
        }
        return;
    }

    const result = await fetchPageContent(pageData);
    if (result.ok) {
        setCachedContent(pageData.slug, result.html);
    }
    await applyContentAndSetup(pageElement, pageContentElement, result.html);
}

setupPage();

// Setup functions to run after widget refresh
function setupRefreshedWidget(widgetElement) {
    // Setup lazy images within the refreshed widget
    const images = widgetElement.querySelectorAll("img[loading=lazy]");
    for (let i = 0; i < images.length; i++) {
        const image = images[i];
        if (image.complete) {
            image.classList.add("cached", "finished-transition");
        } else {
            image.addEventListener("load", () => {
                image.classList.add("loaded");
                setTimeout(() => image.classList.add("finished-transition"), 400);
            });
        }
    }

    // Setup carousels within the refreshed widget
    const carouselElements = widgetElement.querySelectorAll(".carousel-container");
    for (let i = 0; i < carouselElements.length; i++) {
        const carousel = carouselElements[i];
        carousel.classList.add("show-right-cutoff");
        const itemsContainer = carousel.querySelector(".carousel-items-container");
        if (!itemsContainer) continue;

        const determineSideCutoffs = () => {
            if (itemsContainer.scrollLeft != 0) {
                carousel.classList.add("show-left-cutoff");
            } else {
                carousel.classList.remove("show-left-cutoff");
            }
            if (Math.ceil(itemsContainer.scrollLeft) + itemsContainer.clientWidth < itemsContainer.scrollWidth) {
                carousel.classList.add("show-right-cutoff");
            } else {
                carousel.classList.remove("show-right-cutoff");
            }
        };
        const determineSideCutoffsRateLimited = throttledDebounce(determineSideCutoffs, 20, 100);
        itemsContainer.addEventListener("scroll", determineSideCutoffsRateLimited);
        window.addEventListener("resize", determineSideCutoffsRateLimited);
        afterContentReady(determineSideCutoffs);
    }

    // Setup collapsible lists within the refreshed widget
    const collapsibleLists = widgetElement.querySelectorAll(".list.collapsible-container");
    for (let i = 0; i < collapsibleLists.length; i++) {
        const list = collapsibleLists[i];
        if (list.dataset.collapseAfter === undefined) continue;
        const collapseAfter = parseInt(list.dataset.collapseAfter);
        if (collapseAfter == -1 || list.children.length <= collapseAfter) continue;
        
        attachExpandToggleButton(list);
        for (let c = collapseAfter; c < list.children.length; c++) {
            const child = list.children[c];
            child.classList.add("collapsible-item");
            child.style.animationDelay = ((c - collapseAfter) * 20).toString() + "ms";
        }
    }

    // Setup collapsible grids within the refreshed widget
    const collapsibleGridElements = widgetElement.querySelectorAll(".cards-grid.collapsible-container");
    for (let i = 0; i < collapsibleGridElements.length; i++) {
        const gridElement = collapsibleGridElements[i];
        if (gridElement.dataset.collapseAfterRows === undefined) continue;
        const collapseAfterRows = parseInt(gridElement.dataset.collapseAfterRows);
        if (collapseAfterRows == -1) continue;

        const getCardsPerRow = () => parseInt(getComputedStyle(gridElement).getPropertyValue('--cards-per-row'));
        const button = attachExpandToggleButton(gridElement);
        let cardsPerRow;

        const resolveCollapsibleItems = () => requestAnimationFrame(() => {
            const hideItemsAfterIndex = cardsPerRow * collapseAfterRows;
            if (hideItemsAfterIndex >= gridElement.children.length) {
                button.style.display = "none";
            } else {
                button.style.removeProperty("display");
            }
            let row = 0;
            for (let i = 0; i < gridElement.children.length; i++) {
                const child = gridElement.children[i];
                if (i >= hideItemsAfterIndex) {
                    child.classList.add("collapsible-item");
                    child.style.animationDelay = (row * 40).toString() + "ms";
                    if (i % cardsPerRow + 1 == cardsPerRow) row++;
                } else {
                    child.classList.remove("collapsible-item");
                    child.style.removeProperty("animation-delay");
                }
            }
        });

        const observer = new ResizeObserver(() => {
            if (!isElementVisible(gridElement)) return;
            const newCardsPerRow = getCardsPerRow();
            if (cardsPerRow == newCardsPerRow) return;
            cardsPerRow = newCardsPerRow;
            resolveCollapsibleItems();
        });

        cardsPerRow = getCardsPerRow();
        resolveCollapsibleItems();
        afterContentReady(() => observer.observe(gridElement));
    }

    // Setup dynamic relative time within the refreshed widget
    const relativeTimeElements = widgetElement.querySelectorAll("[data-dynamic-relative-time]");
    if (relativeTimeElements.length > 0) {
        updateRelativeTimeForElements(relativeTimeElements);
    }

    // Re-run global setup that's safe to call multiple times
    setupPopovers();
    setupMasonries();
}

// Widget click-to-refresh handler
document.addEventListener('click', async function(e) {
    const target = e.target.closest('.widget-refresh-title');
    if (target && target.dataset.widgetId) {
        e.preventDefault();
        const widgetId = target.dataset.widgetId;
        const widgetElement = target.closest('.widget');
        
        if (!widgetElement) return;
        
        // Add loading state
        widgetElement.style.opacity = '0.5';
        widgetElement.style.pointerEvents = 'none';
        
        try {
            const base = pageData.basePath || '';
            const response = await fetch(`${base}/api/widgets/${widgetId}/`, {
                method: 'GET',
                headers: { 'Accept': 'text/html' }
            });
            
            if (response.ok) {
                const html = await response.text();
                const tempDiv = document.createElement('div');
                tempDiv.innerHTML = html;
                const newWidget = tempDiv.firstElementChild;
                
                if (newWidget) {
                    widgetElement.replaceWith(newWidget);
                    setupRefreshedWidget(newWidget);
                }
            } else {
                console.error('Failed to refresh widget:', response.status);
                widgetElement.style.opacity = '1';
                widgetElement.style.pointerEvents = '';
            }
        } catch (err) {
            console.error('Error refreshing widget:', err);
            widgetElement.style.opacity = '1';
            widgetElement.style.pointerEvents = '';
        }
    }
});
