function renderPie(config, svg, W, H, create) {
    const rows = config.data;
    if (!rows.length) return;
    const total = rows.reduce((sum, row) => sum + row.value, 0);
    if (!(total > 0)) return;

    const namespace = svg.namespaceURI;
    const style = chartStyle;
    const identity = 'matrix(1,0,0,1,0,0)';
    const colors = ['rgb(11,111,206)', 'rgb(120,201,83)'];
    const markerColors = ['#0B6FCE', '#78C953'];
    const sprite = number => 'vfext4-ext-sprite-' + number;
    const legendBase = 1035;
    let nextSprite = legendBase + (config.legend ? 1 + 4 * rows.length : 0);

    function text(tagAttributes, label, dy) {
        const element = create('text', tagAttributes);
        const span = document.createElementNS(namespace, 'tspan');
        span.setAttribute('x', tagAttributes.x);
        span.setAttribute('dy', dy);
        span.textContent = label;
        element.appendChild(span);
        return element;
    }

    // Measure the same SVG text used for the legend; its hit area includes the
    // marker and the text's ascent above the row's origin.
    const legendItems = config.legend ? rows.map(row => {
        const label = String(row.label);
        const element = text({
            x: 0, y: 0, font: '12px Helvetica, sans-serif',
            style: style + ' font: 12px Helvetica, sans-serif;',
            'text-anchor': 'start'
        }, label, 3.5);
        const bounds = element.getBBox();
        element.remove();
        const top = Math.min(0, bounds.y);
        const bottom = Math.max(12, bounds.y + bounds.height);
        return {label: label, width: 20 + bounds.width, top: top, height: bottom - top};
    }) : [];
    const legendWidth = config.legend ? Math.round(Math.max(...legendItems.map(item => item.width)) + 10) : 0;
    const legendStep = config.legend ? Math.ceil(Math.max(...legendItems.map(item => item.height))) + 5 : 0;
    const legendHeight = config.legend ? rows.length * legendStep + 10 : 0;
    const cx = (W - (config.legend ? legendWidth + 10 : 0)) / 2;
    const cy = H / 2;
    const radius = Math.max(0, H / 2 - 10);
    // Form weighted offsets in thousandths of a percent of a turn before
    // converting to radians. Endpoints and centers share that conversion;
    // retain fractional units rather than rounding arbitrary input weights.
    const turnUnits = 100000;
    const radiansPerUnit = Math.PI * 2 / turnUnits;
    const weights = rows.map(row => row.value / total * turnUnits);
    const rotation = weights[0] / 2;
    let cumulative = 0;
    const segments = rows.map((row, index) => {
        const weight = weights[index];
        const start = (rotation - cumulative - weight) * radiansPerUnit;
        const end = (rotation - cumulative) * radiansPerUnit;
        const middle = (rotation - cumulative - weight / 2) * radiansPerUnit;
        cumulative += weight;
        const sx = cx + radius * Math.cos(start);
        const sy = cy + radius * Math.sin(start);
        const mx = cx + radius * Math.cos(middle);
        const my = cy + radius * Math.sin(middle);
        const ex = cx + radius * Math.cos(end);
        const ey = cy + radius * Math.sin(end);
        const d = 'M' + cx + ',' + cy + 'L' + sx + ',' + sy +
            'A' + radius + ',' + radius + ',0,0,1,' + mx + ',' + my +
            'L' + mx + ',' + my +
            'A' + radius + ',' + radius + ',0,0,1,' + ex + ',' + ey +
            'L' + ex + ',' + ey + 'L' + cx + ',' + cy + 'Z';
        return {path: d, angle: middle, label: String(row.label)};
    });

    // Shadow layers precede the filled sectors so their strokes remain behind
    // neighboring slices. Adjacent path identities advance by two.
    const shadows = [
        {stroke: 'rgb(200, 200, 200)', width: 6, x: 1.2, y: 2},
        {stroke: 'rgb(150, 150, 150)', width: 4, x: 0.9, y: 1.5},
        {stroke: 'rgb(100, 100, 100)', width: 2, x: 0.6, y: 1}
    ];
    segments.forEach(segment => shadows.forEach(shadow => {
        create('path', {
            id: sprite(nextSprite), d: segment.path, fill: 'none', hidden: 0,
            stroke: shadow.stroke, 'stroke-width': shadow.width,
            'stroke-linejoin': 'round', 'stroke-opacity': 1,
            style: style, transform: 'matrix(1,0,0,1,' + shadow.x + ',' + shadow.y + ')',
            zIndex: 0
        });
        nextSprite += 2;
    }));
    segments.forEach((segment, index) => {
        create('path', {
            id: sprite(nextSprite), d: segment.path,
            fill: colors[index % colors.length], hidden: 0,
            stroke: colors[0], 'stroke-width': '0px',
            style: style, transform: identity, zIndex: 0
        });
        nextSprite += 2;
    });
    segments.forEach(segment => {
        const x = cx + radius / 2 * Math.cos(segment.angle);
        const y = cy + radius / 2 * Math.sin(segment.angle);
        text({
            id: sprite(nextSprite++), x: x, y: y,
            fill: '#333', font: '11px Helvetica, sans-serif',
            style: style + ' font: 11px Helvetica, sans-serif;',
            text: segment.label, 'text-anchor': 'middle',
            transform: identity, zIndex: 0
        }, segment.label, 3.25);
    });

    if (!config.legend) return;
    const legendX = W - legendWidth - 5;
    const legendY = H / 2 - legendHeight / 2 + 9;
    create('rect', {
        id: sprite(legendBase), x: W - legendWidth - 10.5,
        y: H / 2 - legendHeight / 2 - 0.5,
        width: legendWidth, height: legendHeight, rx: 0, ry: 0,
        fill: '#FFF', stroke: '#000', 'stroke-width': 1, hidden: false,
        style: style, transform: identity, zIndex: 100
    });
    legendItems.forEach((item, index) => {
        create('rect', {
            id: sprite(legendBase + 4 + index * 4),
            x: 0, y: item.top, width: item.width, height: item.height,
            rx: 0, ry: 0, fill: '#FFF',
            style: style + ' cursor: pointer;',
            transform: 'matrix(1,0,0,1,' + legendX + ',' + (legendY + index * legendStep) + ')',
            zIndex: 501
        });
    });
    legendItems.forEach((item, index) => {
        const y = legendY + index * legendStep;
        text({
            id: sprite(legendBase + 2 + index * 4), x: legendX + 20, y: y + 6,
            fill: '#000', font: '12px Helvetica, sans-serif', text: item.label,
            'text-anchor': 'start',
            style: style + ' font: 12px Helvetica, sans-serif; cursor: pointer;',
            transform: identity, zIndex: 502
        }, item.label, 3.5);
        create('rect', {
            id: sprite(legendBase + 3 + index * 4),
            x: 0, y: 0, width: 12, height: 12, rx: 0, ry: 0,
            fill: markerColors[index % markerColors.length], hidden: false,
            style: style + ' cursor: pointer;',
            transform: 'matrix(1,0,0,1,' + legendX + ',' + y + ')', zIndex: 502
        });
    });
}
