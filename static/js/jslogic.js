var jsonActiveItem = 0;
var json = [];

function updateSearchResults(ai) {
    const resultsContainer = document.getElementById("resultsul");
    resultsContainer.innerHTML = ""; // Clear previous results
    json.forEach(result => {
        const listItem = document.createElement("li");
        listItem.className = "p-1";
        // add a element href to the list item
        const link = document.createElement("a");
        link.href = `${result.action}`;
        link.textContent = `${result.text}`;
        listItem.appendChild(link);
        resultsContainer.appendChild(listItem);
    });
    // if there are no results, show "No results found"
    if (json.length === 0) {
        const listItem = document.createElement("li");
        listItem.className = "p-1";
        listItem.textContent = "Keine Ergebnisse gefunden";
        resultsContainer.appendChild(listItem);
    }

    // add classes bg-accent-green font-bold rounded to the active item
    const activeItem = resultsContainer.children[ai];
    if (activeItem) {
        activeItem.classList.add("bg-accent-green", "font-bold", "rounded");
    }
    // remove classes bg-accent-green font-bold rounded from all other items
    for (let i = 0; i < resultsContainer.children.length; i++) {
        if (i !== ai) {
            resultsContainer.children[i].classList.remove("bg-accent-green", "font-bold", "rounded");
        }
    }
}

function searchhandler(e) {
    const  value = e.target.value;

    // if jsonActiveItem is not defined, set it to 0
    if (typeof jsonActiveItem === 'undefined') {
        jsonActiveItem = 0;
    }
    if (typeof json === 'undefined') {
        json = [];
    }
    const keyDown = 40;
    const keyUp = 38;
    const keyReturn = 13;
    const keyEscape = 27;
    const keyTab = 9;


    // get the keyCode of the pressed key
    if (e.keyCode) {
        // for IE and other browsers
        e.keyCode = e.keyCode;
    } else if (e.which) {
        // for Firefox and other browsers
        e.keyCode = e.which;
    } else {
        // for Edge and other browsers
        e.keyCode = e.key;
    }
    const searchResult = document.getElementById("searchresult");

    // when key down or up, we do not want to submit the form
    if (e.keyCode === keyDown || e.keyCode === keyUp) {
        e.preventDefault();
        // we want to change the active item in the search results
        if (e.keyCode === keyDown) {
            // move down in the list
            if (jsonActiveItem < json.length - 1) {
                // only increase if we are not at the last item
                jsonActiveItem = jsonActiveItem + 1;
            }
        } else if (e.keyCode === keyUp) {
            // move up in the list
            if (jsonActiveItem > 0) {
                // only decrease if we are not at the first item
                jsonActiveItem = jsonActiveItem - 1;
            }
        }
    } else if (e.keyCode === keyTab) {
        searchResult.classList.add("hidden");
        return;
    } else if (e.keyCode === keyReturn) {
        // when return is pressed, we want change the location to the active item
        e.preventDefault();
        if (jsonActiveItem < json.length && jsonActiveItem >= 0) {
            // only change the location if the active item is valid
            const activeItem = json[jsonActiveItem];
            if (activeItem && activeItem.action) {
                window.location.href = activeItem.action;
            }
        }
        return;
    } else if (e.keyCode === keyEscape) {
        // when escape is pressed, we want to close the search results
        searchResult.classList.add("hidden");
        jsonActiveItem = 0; // Reset active item
        return;
    } else {
        jsonActiveItem = 0; // Reset active item when typing a new search
        fetch('/search?query=' + encodeURIComponent(value))
          .then(r => r.json())
          .then(data => { json = data; updateSearchResults(0); });
    }

    if (value.length > 0) {
        searchResult.classList.remove("hidden");
    } else {
        searchResult.classList.add("hidden");
    }
    updateSearchResults(jsonActiveItem);
}