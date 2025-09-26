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

function invoiceStatusPanel({ id, status, csrf }) {
  return {
    id,
    status,
    csrf,
    loading: false,
    confirmVoid: false,
    issuedAt: '',
    paidAt: '',
    voidedAt: '',

    get statusLabel() {
      switch (this.status) {
        case 'draft':  return 'Entwurf';
        case 'issued': return 'Gestellt';
        case 'paid':   return 'Bezahlt';
        case 'voided': return 'Storniert';
        default:       return this.status;
      }
    },
    get badgeClass() {
      return {
        'bg-yellow-100 text-yellow-800': this.status === 'draft',
        'bg-blue-100 text-blue-800':     this.status === 'issued',
        'bg-green-100 text-green-800':   this.status === 'paid',
        'bg-red-100 text-red-800':       this.status === 'voided',
      };
    },

    async setStatus(next) {
      if (this.loading) return;
      this.loading = true;

      try {
        const body = new URLSearchParams({ status: next, csrf: this.csrf });
        const res = await fetch(`/invoice/status/${this.id}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/x-www-form-urlencoded', 'X-Requested-With': 'fetch' },
          body
        });

        // Bei Redirect einfach folgen (Server kann nach PRG-Pattern umleiten)
        if (res.redirected) {
          window.location.href = res.url;
          return;
        }

        // Erwartet 200/204 – dann UI aktualisieren (optimistisch) oder Seite neu laden
        if (res.ok) {
          this.status = next;
          const now = new Date().toLocaleDateString(); // einfache Anzeige; Server rendert beim Reload korrekt
          if (next === 'issued') this.issuedAt = now;
          if (next === 'paid')   this.paidAt = now;
          if (next === 'voided') this.voidedAt = now;
          this.confirmVoid = false;
        } else {
          // Fallback: Seite neu laden, damit Fehlermeldung/Flash sichtbar wird
          window.location.reload();
        }
      } catch (e) {
        window.location.reload();
      } finally {
        this.loading = false;
      }
    }
  };
}
