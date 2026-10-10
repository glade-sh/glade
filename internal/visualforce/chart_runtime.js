// Independently draw the default SVG chart profiles observed by the API59/67
// presentation controls. Geometry uses actual data and SVG text measurements;
// no captured DOM, project names, or case identifiers enter this runtime.
const chartNamespace = 'http://www.w3.org/2000/svg';
const chartStyle = '-webkit-tap-highlight-color: rgba(0, 0, 0, 0);';
const chartIdentity = 'matrix(1,0,0,1,0,0)';
const chartBlue = 'rgb(11,111,206)';
const chartFont = '12px Arial, Helvetica, sans-serif';

function renderCartesian(config, svg, W, H, create) {
    const rows = config.data;
    const minimum = Math.min(...rows.map(row => row.value));
    const maximum = Math.max(...rows.map(row => row.value));
    const range = maximum - minimum;
    if (!(range > 0)) return;

    function measure(label, font) {
        const text = create('text', font ? {style: 'font: ' + font + ';'} : {});
        text.textContent = label;
        const width = text.getBBox().width;
        text.remove();
        return width;
    }
    function label(id, value, x, y) {
        const text = create('text', {
            id: 'vfext4-ext-sprite-' + id,
            fill: '#444', font: chartFont, text: value,
            'text-anchor': 'start', x: x, y: y,
            style: chartStyle + ' font: ' + chartFont + ';',
            transform: chartIdentity, zIndex: 0
        });
        const span = document.createElementNS(chartNamespace, 'tspan');
        span.setAttribute('x', x);
        span.setAttribute('dy', 3.5);
        span.textContent = value;
        text.appendChild(span);
    }
    const ticks = Array.from({length: 6}, (_, i) => {
        // Decimal ticks are labels, rather than binary arithmetic residues.
        const value = Number((minimum + range * i / 5).toPrecision(12));
        const text = String(value);
        return {text: text, width: measure(text, chartFont)};
    });
    const widest = Math.max(...ticks.map(tick => tick.width));
    const left = widest + 18;
    const right = W - 10;
    const top = 10;
    const bottom = H - 32;
    const plotHeight = bottom - top;
    // Public bar-series default gutter is 38.2% of a bar width. Ten pixels
    // of padding sit on either side of the category positions.
    const gutter = 0.382;
    const barWidth = (right - left - 20) / (rows.length * (1 + gutter) - gutter);
    const centers = rows.map((row, i) => config.kind === 'bar'
        ? left + 10 + barWidth / 2 + i * barWidth * (1 + gutter)
        : left + (right - left) * i / (rows.length - 1));
    let next = config.kind === 'line' ? 1039 : 1038;
    function axis(path) {
        create('path', {
            id: 'vfext4-ext-sprite-' + next++, d: path, fill: 'none',
            stroke: '#444', 'stroke-width': 1, style: chartStyle,
            transform: chartIdentity, zIndex: 0
        });
    }
    let horizontal = 'M' + left + ',' + (Math.floor(bottom) + 0.5) +
        'L' + right + ',' + (Math.floor(bottom) + 0.5);
    centers.forEach(x => {
        const snap = Math.floor(x) + 0.5;
        horizontal += 'M' + snap + ',' + bottom + 'L' + snap + ',' + (bottom + 7);
    });
    axis(horizontal);
    rows.forEach((row, i) => {
        // Category placement uses the unstyled SVG label's bounding box;
        // painting uses the axis font, as observed in the native controls.
        const x = Math.round(centers[i] - measure(row.label) / 2);
        label(next++, row.label, x, bottom + 18);
    });
    let vertical = 'M' + (Math.floor(left) + 0.5) + ',' + bottom +
        'L' + (Math.floor(left) + 0.5) + ',' + top;
    ticks.forEach((tick, i) => {
        tick.y = Math.floor(bottom - plotHeight * i / (ticks.length - 1));
        vertical += 'M' + (left - 6) + ',' + (tick.y + 0.5) +
            'L' + (left + 1) + ',' + (tick.y + 0.5);
    });
    axis(vertical);
    ticks.forEach(tick => label(next++, tick.text, left - widest + 6 - tick.width, tick.y));

    if (config.kind === 'bar') {
        const shadows = [
            {stroke: 'rgb(200,200,200)', opacity: 0.05, width: '6px', offset: 1.2},
            {stroke: 'rgb(150,150,150)', opacity: 0.1, width: '4px', offset: 0.9},
            {stroke: 'rgb(100,100,100)', opacity: 0.15, width: '2px', offset: 0.6}
        ];
        rows.forEach((row, i) => {
            const height = Math.floor((row.value - minimum) / range * plotHeight);
            const rect = {
                x: Math.round(left + 10 + i * barWidth * (1 + gutter)),
                y: bottom - height, width: Math.floor(barWidth), height: height,
                rx: 0, ry: 0, style: chartStyle, zIndex: 0
            };
            shadows.forEach(shadow => {
                create('rect', {
                    ...rect, id: 'vfext4-ext-sprite-' + next,
                    fill: 'none', hidden: 0, stroke: shadow.stroke,
                    'stroke-opacity': shadow.opacity, 'stroke-width': shadow.width,
                    transform: 'matrix(1,0,0,1,' + shadow.offset + ',' + shadow.offset + ')'
                });
                next += 2;
            });
            create('rect', {
                ...rect, id: 'vfext4-ext-sprite-' + next,
                fill: chartBlue, hidden: false, stroke: chartBlue,
                'stroke-width': '0px', transform: chartIdentity
            });
            next += 2;
        });
        return;
    }

    // With two points the smooth curve's endpoint controls coincide with the
    // endpoints. Preserve the captured two-decimal coordinate serialization.
    const coordinates = rows.map((row, i) => ({
        x: Number(centers[i].toFixed(2)),
        y: Number((bottom - (row.value - minimum) / range * plotHeight).toFixed(2))
    }));
    const first = coordinates[0], last = coordinates[coordinates.length - 1];
    const path = 'M' + first.x + ',' + first.y + 'C' + first.x + ',' + first.y +
        ',' + last.x + ',' + last.y + ',' + last.x + ',' + last.y;
    const markerBase = next;
    const lineID = next + rows.length;
    [6, 4, 2].forEach((width, i) => create('path', {
        id: 'vfext4-ext-sprite-' + (lineID + 1 + i), d: path,
        fill: 'none', hidden: false, stroke: 'rgb(0, 0, 0)',
        'stroke-opacity': [0.05, 0.1, 0.15][i], 'stroke-width': width,
        style: chartStyle, transform: 'matrix(1,0,0,1,1,1)', zIndex: 0
    }));
    create('path', {
        id: 'vfext4-ext-sprite-' + lineID, d: path,
        fill: 'none', hidden: false, opacity: 1, stroke: chartBlue,
        'stroke-width': '0.5px', style: chartStyle,
        transform: chartIdentity, zIndex: 3000
    });
    coordinates.forEach((point, i) => create('circle', {
        id: 'vfext4-ext-sprite-' + (markerBase + i),
        cx: 0, cy: 0, r: 3, x: 0, y: 0, fill: chartBlue, hidden: false,
        stroke: chartBlue, style: chartStyle,
        transform: 'matrix(1,0,0,1,' + point.x + ',' + point.y + ')', zIndex: 4000
    }));
}

window.GladeVisualforceCharts = function (id, config) {
    const root = document.getElementById(id);
    const surface = root && root.querySelector('.vf-surface');
    if (!surface) return;
    const W = surface.getBoundingClientRect().width;
    const H = config.height;
    const svg = document.createElementNS(chartNamespace, 'svg');
    const attributes = {
        id: 'vfext4-ext-gen2', width: W, height: H,
        version: '1.1', xmlns: chartNamespace,
        style: 'width: ' + W + 'px; height: ' + H + 'px;'
    };
    Object.entries(attributes).forEach(([name, value]) => svg.setAttribute(name, value));
    surface.appendChild(svg);
    const create = (tag, attrs) => {
        const element = document.createElementNS(chartNamespace, tag);
        Object.entries(attrs).forEach(([name, value]) => element.setAttribute(name, value));
        svg.appendChild(element);
        return element;
    };
    create('defs', {});
    create('rect', {
        id: 'vfext4-ext-gen3', fill: '#000', height: '100%', width: '100%',
        stroke: 'none', opacity: 0
    });
    if (config.kind === 'pie') renderPie(config, svg, W, H, create);
    else renderCartesian(config, svg, W, H, create);
};
